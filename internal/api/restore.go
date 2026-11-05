package api

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fhavrlent/wherehouse/internal/i18n"
	"github.com/fhavrlent/wherehouse/internal/store"
)

// maxBackupBytes is the largest backup that can be restored.
const maxBackupBytes = 8 << 30

// restoreResult says what a restore brought back.
type restoreResult struct {
	HouseholdID int64  `json:"householdId"`
	Name        string `json:"name"`
	Things      int    `json:"things"`
	Photos      int    `json:"photos"`
	Documents   int    `json:"documents"`
	// LabelsSkipped are labels already on things of another household here,
	// e.g. the one the backup was made of; their things are restored without.
	LabelsSkipped int `json:"labelsSkipped"`
}

// restoreZip makes a new household from a backup made with export.zip, with
// everything in it, and switches to it. Nothing that exists is changed.
func (s *Server) restoreZip(w http.ResponseWriter, r *http.Request) {
	// The archive is read from a file: zip needs to seek.
	tmp, err := os.CreateTemp(s.cfg.DataDir, ".restore-*.zip")
	if err != nil {
		fail(w, err)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	n, err := io.Copy(tmp, http.MaxBytesReader(w, r.Body, maxBackupBytes))
	if err != nil {
		writeMsg(w, http.StatusRequestEntityTooLarge, "server.backupTooLarge", nil)
		return
	}
	zr, err := zip.NewReader(tmp, n)
	if err != nil {
		writeMsg(w, http.StatusBadRequest, "server.backupInvalid", nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Minute)
	defer cancel()
	res, err := s.restore(ctx, userID(r), zr)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

// restorer brings one backup back into a new household.
type restorer struct {
	s       *Server
	uid, hh int64
	zr      *zip.Reader
	b       backup
	// saved are the files written so far, removed if the restore fails.
	saved []string
	// Old IDs to new ones.
	kinds, fields, icons, things map[int64]int64
	res                          restoreResult
}

func (s *Server) restore(ctx context.Context, uid int64, zr *zip.Reader) (*restoreResult, error) {
	rs := &restorer{s: s, uid: uid, zr: zr, kinds: map[int64]int64{}, fields: map[int64]int64{},
		icons: map[int64]int64{}, things: map[int64]int64{}}
	f, err := rs.open("wherehouse.json")
	if err != nil {
		return nil, badBackup()
	}
	err = json.NewDecoder(f).Decode(&rs.b)
	f.Close()
	if err != nil || rs.b.Format != 1 || rs.b.Household == nil {
		return nil, badBackup()
	}
	hh, err := s.store.CreateHousehold(ctx, uid, rs.b.Household.Name, rs.b.Household.Currency)
	if err != nil {
		return nil, err
	}
	rs.hh = hh
	if err := rs.run(ctx); err != nil {
		// Nothing half-restored stays.
		files, derr := s.store.DropHousehold(context.WithoutCancel(ctx), hh)
		if derr != nil {
			slog.Error("restore: remove the unfinished household", "household", hh, "err", derr)
		}
		s.files.Remove(append(files, rs.saved...)...)
		return nil, err
	}
	rs.res.HouseholdID, rs.res.Name = hh, rs.b.Household.Name
	slog.Info("restored a backup", "household", hh, "things", rs.res.Things, "photos", rs.res.Photos,
		"documents", rs.res.Documents, "labels skipped", rs.res.LabelsSkipped)
	return &rs.res, nil
}

func badBackup() error { return i18n.NewError("server.backupInvalid", nil) }

func (rs *restorer) open(name string) (io.ReadCloser, error) {
	for _, f := range rs.zr.File {
		if f.Name == name {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("%s isn't in the backup", name)
}

// saveFile copies a file out of the archive into the data folder.
func (rs *restorer) saveFile(path string) (name string, size int64, mimeType string, err error) {
	src, err := rs.open(path)
	if err != nil {
		return "", 0, "", badBackup()
	}
	defer src.Close()
	limit := rs.s.cfg.MaxUploadMB << 20
	head := make([]byte, 512)
	n, _ := io.ReadFull(src, head)
	mimeType = detectType(head[:n], path)
	body := io.MultiReader(strings.NewReader(string(head[:n])), src)
	name, size, err = rs.s.files.Save(io.LimitReader(body, limit+1), strings.ToLower(filepath.Ext(path)))
	if err != nil {
		return "", 0, "", err
	}
	rs.saved = append(rs.saved, name)
	if size > limit {
		return "", 0, "", i18n.NewError("server.backupFileTooLarge", i18n.Args{"mb": rs.s.cfg.MaxUploadMB})
	}
	return name, size, mimeType, nil
}

func (rs *restorer) run(ctx context.Context) error {
	st, b, hh := rs.s.store, &rs.b, rs.hh
	for _, i := range b.Icons {
		file, _, mimeType, err := rs.saveFile(i.Path)
		if err != nil {
			return err
		}
		icon, err := st.AddIcon(ctx, hh, file, mimeType)
		if err != nil {
			return err
		}
		rs.icons[i.ID] = icon.ID
	}
	if err := rs.restoreKinds(ctx); err != nil {
		return err
	}
	fields := slices.Clone(b.Fields)
	slices.SortStableFunc(fields, func(a, b store.Field) int { return a.Position - b.Position })
	for _, f := range fields {
		nf, err := st.CreateField(ctx, hh, f.Name, f.Type)
		if err != nil {
			return err
		}
		rs.fields[f.ID] = nf.ID
	}
	// What things are in comes first.
	things := slices.Clone(b.Things)
	slices.SortStableFunc(things, func(a, b store.Thing) int {
		if d := len(a.Path) - len(b.Path); d != 0 {
			return d
		}
		return int(a.ID - b.ID)
	})
	for i := range things {
		if err := rs.restoreThing(ctx, &things[i]); err != nil {
			return fmt.Errorf("%s: %w", things[i].Name, err)
		}
	}
	// Tags were added by name with the things; they get their colours back.
	tags, err := st.ListTags(ctx, hh)
	if err != nil {
		return err
	}
	for _, t := range tags {
		for _, old := range b.Tags {
			if old.Name == t.Name && old.Color != t.Color {
				if err := st.UpdateTag(ctx, hh, t.ID, t.Name, old.Color); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// icon is a kind's icon, with an uploaded one's ID as it is now.
func (rs *restorer) icon(icon string) string {
	if id, ok := strings.CutPrefix(icon, "custom:"); ok {
		old, _ := strconv.ParseInt(id, 10, 64)
		if n, ok := rs.icons[old]; ok {
			return "custom:" + strconv.FormatInt(n, 10)
		}
		return ""
	}
	return icon
}

func (rs *restorer) restoreKinds(ctx context.Context) error {
	st := rs.s.store
	have, err := st.ListKinds(ctx, rs.hh)
	if err != nil {
		return err
	}
	builtin := map[string]int64{}
	for _, k := range have {
		builtin[k.Key] = k.ID
	}
	for _, k := range rs.b.Kinds {
		in := store.KindInput{Name: k.Name, Icon: rs.icon(k.Icon), IsPlace: k.IsPlace, Position: k.Position}
		if id, ok := builtin[k.Key]; ok && k.Key != "" {
			if _, err := st.UpdateKind(ctx, rs.hh, id, in); err != nil {
				return err
			}
			rs.kinds[k.ID] = id
			continue
		}
		nk, err := st.CreateKind(ctx, rs.hh, in)
		if err != nil {
			return err
		}
		rs.kinds[k.ID] = nk.ID
	}
	return nil
}

func (rs *restorer) restoreThing(ctx context.Context, t *store.Thing) error {
	st, hh := rs.s.store, rs.hh
	in := store.ThingInput{KindID: rs.kinds[t.Kind.ID], Name: t.Name, Description: t.Description, Quantity: t.Quantity,
		Manufacturer: t.Manufacturer, Model: t.Model, Serial: t.Serial, Notes: t.Notes, Purchase: t.Purchase,
		Fields: map[int64]string{}}
	if t.ParentID != nil {
		parent, ok := rs.things[*t.ParentID]
		if !ok {
			return badBackup()
		}
		in.ParentID = &parent
	}
	tags := make([]string, len(t.Tags))
	for i, tag := range t.Tags {
		tags[i] = tag.Name
	}
	in.Tags = &tags
	for _, v := range t.Fields {
		if id, ok := rs.fields[v.FieldID]; ok {
			in.Fields[id] = v.Value
		}
	}
	nt, err := st.CreateThing(ctx, hh, rs.uid, in)
	if err != nil {
		return err
	}
	rs.things[t.ID] = nt.ID
	rs.res.Things++
	if t.Label != "" {
		if _, err := st.AttachLabel(ctx, hh, nt.ID, t.Label); err != nil {
			var ve *store.ValidationError
			if !errors.As(err, &ve) || ve.Key != "server.labelUsed" {
				return err
			}
			rs.res.LabelsSkipped++
		}
	}
	for _, l := range t.Links {
		if l.Source == "" {
			if _, err := st.AddLink(ctx, hh, nt.ID, l.URL, l.Title); err != nil {
				return err
			}
			continue
		}
		link := store.AppLinkInput{ExternalID: l.ExternalID, URL: l.URL, Title: l.Title, Data: l.Data}
		if t.PurchaseOwner == l.Source {
			p := t.Purchase
			link.Purchase = &p
		}
		if err := st.SetAppLink(ctx, hh, nt.ID, l.Source, link); err != nil {
			return err
		}
	}
	for _, p := range t.Photos {
		path, ok := rs.b.Photos[p.ID]
		if !ok {
			continue
		}
		file, size, mimeType, err := rs.saveFile(path)
		if err != nil {
			return err
		}
		np := &store.Photo{ThingID: nt.ID, File: file, MimeType: mimeType, Size: size}
		if err := st.AddPhoto(ctx, hh, rs.uid, np); err != nil {
			return err
		}
		rs.res.Photos++
		if t.Cover != nil && *t.Cover == p.ID {
			if err := st.MakeCover(ctx, hh, np.ID); err != nil {
				return err
			}
		}
	}
	for _, a := range t.Attachments {
		na := &store.Attachment{ThingID: nt.ID, Kind: a.Kind, Storage: a.Storage, Filename: a.Filename,
			MimeType: a.MimeType, Size: a.Size, PaperlessDocID: a.PaperlessDocID}
		if a.Storage == store.StoragePaperless {
			na.AddedBy = &rs.uid
		} else {
			path, ok := rs.b.Attachments[a.ID]
			if !ok {
				continue
			}
			if na.File, na.Size, _, err = rs.saveFile(path); err != nil {
				return err
			}
		}
		if err := st.AddAttachment(ctx, hh, na); err != nil {
			return err
		}
		rs.res.Documents++
	}
	for _, m := range t.Maintenance {
		nm, err := st.AddMaintenance(ctx, hh, rs.uid, nt.ID, store.MaintenanceInput{Title: m.Title, DoneOn: m.DoneOn,
			NextDue: m.NextDue, Notes: m.Notes, Cost: m.Cost})
		if err != nil {
			return err
		}
		if err := st.RestoreMaintenanceTimes(ctx, nm.ID, m.CreatedAt, m.UpdatedAt); err != nil {
			return err
		}
	}
	// Last, as adding photos and the rest counts as changing it.
	return st.RestoreTimes(ctx, hh, nt.ID, t.CreatedAt, t.UpdatedAt, t.ArchivedAt)
}
