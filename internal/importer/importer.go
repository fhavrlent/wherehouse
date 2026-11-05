// Package importer copies a HomeBox inventory into a Wherehouse household.
package importer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fhavrlent/wherehouse/internal/homebox"
	"github.com/fhavrlent/wherehouse/internal/store"
)

// Files is where photos and documents are stored.
type Files interface {
	Save(r io.Reader, ext string) (string, int64, error)
	Remove(names ...string)
}

// Progress is how far an import has come. It is safe to copy.
type Progress struct {
	Running     bool     `json:"running"`
	Total       int      `json:"total"`
	Done        int      `json:"done"`
	Things      int      `json:"things"`
	Photos      int      `json:"photos"`
	Documents   int      `json:"documents"`
	Maintenance int      `json:"maintenance"`
	Skipped     int      `json:"skipped"`
	Errors      []string `json:"errors"`
	Error       string   `json:"error,omitempty"`
	Finished    string   `json:"finished,omitempty"`
}

// maxErrors is how many problems with single things are kept to show.
const maxErrors = 50

type Importer struct {
	st    *store.Store
	hb    *homebox.Client
	files Files
	hh    int64
	user  int64
	// kinds are the household's kinds by HomeBox entity type name, and the
	// built-in ones by key.
	kinds map[string]int64

	mu       sync.Mutex
	progress Progress
}

func New(st *store.Store, hb *homebox.Client, files Files, hh, user int64) *Importer {
	return &Importer{st: st, hb: hb, files: files, hh: hh, user: user, kinds: map[string]int64{},
		progress: Progress{Errors: []string{}}}
}

// Progress returns a copy of how far the import has come.
func (im *Importer) Progress() Progress {
	im.mu.Lock()
	defer im.mu.Unlock()
	p := im.progress
	// Never nil: the page reads errors.length, and a nil slice is sent as null.
	p.Errors = append([]string{}, p.Errors...)
	return p
}

func (im *Importer) update(fn func(p *Progress)) {
	im.mu.Lock()
	fn(&im.progress)
	im.mu.Unlock()
}

func (im *Importer) problem(name string, err error) {
	slog.Warn("homebox import", "what", name, "err", err)
	im.update(func(p *Progress) {
		if len(p.Errors) < maxErrors {
			p.Errors = append(p.Errors, fmt.Sprintf("%s: %v", name, err))
		}
	})
}

// Run imports everything. Things imported before are skipped, so running
// it again picks up what was added in HomeBox since, or what failed.
func (im *Importer) Run(ctx context.Context) error {
	im.update(func(p *Progress) { p.Running = true })
	defer im.update(func(p *Progress) {
		p.Running = false
		p.Finished = time.Now().UTC().Format(time.RFC3339)
	})
	err := im.run(ctx)
	if err != nil {
		im.update(func(p *Progress) { p.Error = err.Error() })
	}
	return err
}

func (im *Importer) run(ctx context.Context) error {
	kinds, err := im.st.ListKinds(ctx, im.hh)
	if err != nil {
		return err
	}
	for _, k := range kinds {
		if k.Key != "" {
			im.kinds["key:"+k.Key] = k.ID
		} else {
			im.kinds[strings.ToLower(k.Name)] = k.ID
		}
	}
	tree, err := im.hb.Tree(ctx)
	if err != nil {
		return err
	}
	total := 0
	var count func([]homebox.Node)
	count = func(ns []homebox.Node) {
		for _, n := range ns {
			total++
			count(n.Children)
		}
	}
	count(tree)
	im.update(func(p *Progress) { p.Total = total })
	return im.walk(ctx, tree, nil)
}

func (im *Importer) walk(ctx context.Context, nodes []homebox.Node, parent *int64) error {
	for _, n := range nodes {
		if err := ctx.Err(); err != nil {
			return err
		}
		id, err := im.thing(ctx, n, parent)
		im.update(func(p *Progress) { p.Done++ })
		if err != nil {
			im.problem(n.Name, err)
			continue
		}
		if err := im.walk(ctx, n.Children, &id); err != nil {
			return err
		}
	}
	return nil
}

// kind picks the kind for an entity: its HomeBox type when it has one of
// its own (added here as needed), else building or room for locations and
// a plain thing for items.
func (im *Importer) kind(ctx context.Context, n homebox.Node, e *homebox.Entity, parent *int64) (int64, error) {
	location := n.Type == "location"
	if t := e.EntityType; t != nil && t.Name != "" && !strings.EqualFold(t.Name, "location") && !strings.EqualFold(t.Name, "item") {
		key := strings.ToLower(t.Name)
		if id, ok := im.kinds[key]; ok {
			return id, nil
		}
		icon := "shapes"
		if t.IsLocation {
			icon = "map-pin"
		}
		k, err := im.st.CreateKind(ctx, im.hh, store.KindInput{Name: name(t.Name), Icon: icon, IsPlace: t.IsLocation})
		if err != nil {
			return 0, err
		}
		im.kinds[key] = k.ID
		return k.ID, nil
	}
	switch {
	case location && parent == nil:
		return im.kinds["key:building"], nil
	case location:
		return im.kinds["key:room"], nil
	}
	return im.kinds["key:thing"], nil
}

func (im *Importer) thing(ctx context.Context, n homebox.Node, parent *int64) (int64, error) {
	if id, err := im.st.Imported(ctx, im.hh, n.ID); err != nil || id != 0 {
		if id != 0 {
			im.update(func(p *Progress) { p.Skipped++ })
		}
		return id, err
	}
	e, err := im.hb.Entity(ctx, n.ID)
	if err != nil {
		return 0, err
	}
	kind, err := im.kind(ctx, n, e, parent)
	if err != nil {
		return 0, err
	}
	tags := []string{}
	for _, t := range e.Tags {
		tags = append(tags, t.Name)
	}
	fields, err := im.fields(ctx, e)
	if err != nil {
		return 0, err
	}
	in := store.ThingInput{
		ParentID: parent, KindID: kind, Name: name(e.Name), Description: e.Description,
		Quantity: min(max(e.Quantity, 1), store.MaxQuantity), Manufacturer: e.Manufacturer, Model: e.ModelNumber,
		Serial: e.SerialNumber, Notes: e.Notes, Tags: &tags, Fields: fields,
		Purchase: store.Purchase{
			PurchaseDate: date(e.PurchaseDate), PurchasePrice: price(float64(e.PurchasePrice)), PurchaseFrom: e.PurchaseFrom,
			WarrantyUntil: date(e.WarrantyExpires), LifetimeWarranty: e.LifetimeWarranty, WarrantyDetails: e.WarrantyDetails,
			Insured: e.Insured, SoldDate: date(e.SoldDate), SoldPrice: price(float64(e.SoldPrice)), SoldTo: e.SoldTo,
			SoldNotes: e.SoldNotes,
		},
	}
	t, err := im.st.CreateThing(ctx, im.hh, im.user, in)
	if err != nil {
		return 0, err
	}
	if err := im.st.MarkImported(ctx, im.hh, n.ID, t.ID); err != nil {
		return 0, err
	}
	im.update(func(p *Progress) { p.Things++ })
	if e.Archived {
		if _, err := im.st.Archive(ctx, im.hh, t.ID, true); err != nil {
			return 0, err
		}
	}
	if code := e.AssetCode(); code != "" {
		if _, err := im.st.AttachLabel(ctx, im.hh, t.ID, code); err != nil {
			im.problem(e.Name, fmt.Errorf("label %s: %w", code, err))
		}
	}
	im.copyFiles(ctx, e, t.ID)
	im.maintenance(ctx, e, t.ID)
	return t.ID, nil
}

// fields turns HomeBox's custom fields into custom fields here, adding the
// fields as needed.
func (im *Importer) fields(ctx context.Context, e *homebox.Entity) (map[int64]string, error) {
	out := map[int64]string{}
	for _, f := range e.Fields {
		typ, value := store.FieldText, f.TextValue
		switch f.Type {
		case "number":
			typ, value = store.FieldNumber, number(float64(f.NumberValue))
		case "boolean":
			typ, value = store.FieldBool, ""
			if f.BooleanValue {
				value = "true"
			}
		}
		if value == "" {
			continue
		}
		field, err := im.st.FieldByName(ctx, im.hh, name(f.Name), typ)
		if err != nil {
			return nil, err
		}
		// A field of that name but another type was set up here, which
		// this value may not fit.
		if field.Type != typ && field.Type != store.FieldText {
			continue
		}
		out[field.ID] = value
	}
	return out, nil
}

// copyFiles copies an entity's photos and documents. Problems with single
// files are recorded and the rest go on.
func (im *Importer) copyFiles(ctx context.Context, e *homebox.Entity, thing int64) {
	// The main photo first, so it is the cover here too.
	atts := append([]homebox.Attachment(nil), e.Attachments...)
	for i, a := range atts {
		if a.Type == "photo" && a.Primary && i > 0 {
			atts[0], atts[i] = atts[i], atts[0]
			break
		}
	}
	for _, a := range atts {
		if a.SourceType == "external" || a.SourceType == "link" {
			continue
		}
		if err := im.file(ctx, e.ID, a, thing); err != nil {
			im.problem(e.Name+" / "+a.Title, err)
		}
	}
}

func (im *Importer) file(ctx context.Context, entity string, a homebox.Attachment, thing int64) error {
	res, err := im.hb.OpenAttachment(ctx, entity, a.ID)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	mimeType := a.MimeType
	if mimeType == "" {
		mimeType, _, _ = mime.ParseMediaType(res.Header.Get("Content-Type"))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	filename := a.Title
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			ext = exts[0]
			filename += ext
		}
	}
	file, size, err := im.files.Save(res.Body, ext)
	if err != nil {
		return err
	}
	if a.Type == "photo" && strings.HasPrefix(mimeType, "image/") {
		err = im.st.AddPhoto(ctx, im.hh, im.user, &store.Photo{ThingID: thing, File: file, MimeType: mimeType, Size: size})
		if err == nil {
			im.update(func(p *Progress) { p.Photos++ })
		}
	} else {
		kind := a.Type
		if !store.ValidKind(kind) {
			kind = "other"
		}
		err = im.st.AddAttachment(ctx, im.hh, &store.Attachment{ThingID: thing, Kind: kind, Storage: store.StorageLocal,
			Filename: name(filename), MimeType: mimeType, Size: size, File: file, AddedBy: &im.user})
		if err == nil {
			im.update(func(p *Progress) { p.Documents++ })
		}
	}
	if err != nil {
		im.files.Remove(file)
	}
	return err
}

func (im *Importer) maintenance(ctx context.Context, e *homebox.Entity, thing int64) {
	entries, err := im.hb.Maintenance(ctx, e.ID)
	if err != nil {
		var he *homebox.Error
		if !errors.As(err, &he) || he.Status != http.StatusNotFound {
			im.problem(e.Name+" / maintenance", err)
		}
		return
	}
	for _, m := range entries {
		in := store.MaintenanceInput{Title: name(m.Name), Notes: m.Description}
		if d := homebox.Date(m.CompletedDate); d != "" {
			in.DoneOn = &d
		} else if d := homebox.Date(m.ScheduledDate); d != "" {
			in.NextDue = &d
		} else {
			continue
		}
		if m.Cost > 0 {
			c := float64(m.Cost)
			in.Cost = &c
		}
		if _, err := im.st.AddMaintenance(ctx, im.hh, im.user, thing, in); err != nil {
			im.problem(e.Name+" / "+m.Name, err)
			continue
		}
		im.update(func(p *Progress) { p.Maintenance++ })
	}
}

// name keeps names within what Wherehouse takes; HomeBox allows longer ones.
func name(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "?"
	}
	if r := []rune(s); len(r) > store.MaxName {
		return string(r[:store.MaxName])
	}
	return s
}

func date(s string) *string {
	if d := homebox.Date(s); d != "" {
		return &d
	}
	return nil
}

func price(n float64) *float64 {
	if n <= 0 {
		return nil
	}
	return &n
}

func number(n float64) string {
	if n == 0 {
		return ""
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}
