package api

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fhavrlent/wherehouse/internal/store"
)

// allThings is every thing of the household but templates: current,
// archived and sold, with their details.
func (s *Server) allThings(r *http.Request) ([]store.Thing, error) {
	hh := household(r)
	var out []store.Thing
	for _, archived := range []bool{false, true} {
		list, err := s.store.ListThings(r.Context(), hh, store.Filter{Archived: archived})
		if err != nil {
			return nil, err
		}
		for _, t := range list {
			full, err := s.store.GetThing(r.Context(), hh, t.ID)
			if err != nil {
				return nil, err
			}
			out = append(out, *full)
		}
	}
	return out, nil
}

func exportName(r *http.Request, s *Server, ext string) string {
	name := "wherehouse"
	if h, err := s.store.GetHousehold(r.Context(), household(r)); err == nil {
		name = h.Name
	}
	return fmt.Sprintf("%s %s.%s", name, time.Now().In(s.cfg.Location).Format(store.DateLayout), ext)
}

func optDate(d *string) string {
	if d == nil {
		return ""
	}
	return *d
}

func optNum(n *float64) string {
	if n == nil {
		return ""
	}
	return strconv.FormatFloat(*n, 'f', -1, 64)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return ""
}

// exportCSV writes every thing as a row, with a column per custom field,
// for spreadsheets.
func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	things, err := s.allThings(r)
	if err != nil {
		fail(w, err)
		return
	}
	fields, err := s.store.ListFields(r.Context(), household(r))
	if err != nil {
		fail(w, err)
		return
	}
	kinds, err := s.store.ListKinds(r.Context(), household(r))
	if err != nil {
		fail(w, err)
		return
	}
	kindNames := map[int64]string{}
	for _, k := range kinds {
		kindNames[k.ID] = k.Name
		if k.Name == "" {
			kindNames[k.ID] = k.Key
		}
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": exportName(r, s, "csv")}))
	// A byte order mark, so Excel reads the file as UTF-8.
	io.WriteString(w, "\ufeff")
	cw := csv.NewWriter(w)
	header := []string{"id", "in", "path", "name", "kind", "quantity", "manufacturer", "model", "serial", "description", "notes",
		"purchase_date", "purchase_price", "currency", "purchase_from", "warranty_until", "lifetime_warranty", "warranty_details",
		"insured", "sold_date", "sold_price", "sold_to", "sold_notes", "tags", "label", "archived"}
	for _, f := range fields {
		header = append(header, f.Name)
	}
	cw.Write(header)
	for _, t := range things {
		var path, tags []string
		for _, p := range t.Path {
			path = append(path, p.Name)
		}
		for _, tag := range t.Tags {
			tags = append(tags, tag.Name)
		}
		parent := ""
		if t.ParentID != nil {
			parent = strconv.FormatInt(*t.ParentID, 10)
		}
		row := []string{strconv.FormatInt(t.ID, 10), parent, strings.Join(path, " / "), t.Name, kindNames[t.Kind.ID],
			strconv.Itoa(t.Quantity), t.Manufacturer, t.Model, t.Serial, t.Description, t.Notes,
			optDate(t.PurchaseDate), optNum(t.PurchasePrice), t.Currency, t.PurchaseFrom, optDate(t.WarrantyUntil),
			yesNo(t.LifetimeWarranty), t.WarrantyDetails, yesNo(t.Insured), optDate(t.SoldDate), optNum(t.SoldPrice),
			t.SoldTo, t.SoldNotes, strings.Join(tags, "; "), t.Label, optDate(t.ArchivedAt)}
		values := map[int64]string{}
		for _, v := range t.Fields {
			values[v.FieldID] = v.Value
		}
		for _, f := range fields {
			row = append(row, values[f.ID])
		}
		cw.Write(row)
	}
	cw.Flush()
}

// backup is what export.zip's wherehouse.json holds.
type backup struct {
	Format    int              `json:"format"`
	Exported  string           `json:"exported"`
	Version   string           `json:"version"`
	Household *store.Household `json:"household"`
	Kinds     []store.Kind     `json:"kinds"`
	Icons     []backupFile     `json:"icons"`
	Fields    []store.Field    `json:"fields"`
	Tags      []store.Tag      `json:"tags"`
	Things    []store.Thing    `json:"things"`
	// Files names, by photo, document and icon, where each file is in the
	// archive.
	Photos      map[int64]string `json:"photos"`
	Attachments map[int64]string `json:"attachments"`
}

type backupFile struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
}

// exportZip writes the whole household as an archive: wherehouse.json with
// everything about every thing, and the files of its photos, documents and
// icons. Documents in Paperless stay there and are only listed.
func (s *Server) exportZip(w http.ResponseWriter, r *http.Request) {
	hh := household(r)
	things, err := s.allThings(r)
	if err != nil {
		fail(w, err)
		return
	}
	b := backup{Format: 1, Exported: time.Now().UTC().Format(time.RFC3339), Version: s.version, Things: things,
		Photos: map[int64]string{}, Attachments: map[int64]string{}, Icons: []backupFile{}}
	if b.Household, err = s.store.GetHousehold(r.Context(), hh); err != nil {
		fail(w, err)
		return
	}
	if b.Kinds, err = s.store.ListKinds(r.Context(), hh); err != nil {
		fail(w, err)
		return
	}
	if b.Fields, err = s.store.ListFields(r.Context(), hh); err != nil {
		fail(w, err)
		return
	}
	if b.Tags, err = s.store.ListTags(r.Context(), hh); err != nil {
		fail(w, err)
		return
	}
	icons, err := s.store.ListIcons(r.Context(), hh)
	if err != nil {
		fail(w, err)
		return
	}

	type file struct{ name, path string }
	var files []file
	for _, t := range things {
		for _, p := range t.Photos {
			path := fmt.Sprintf("photos/%d-%d%s", t.ID, p.ID, filepath.Ext(p.File))
			b.Photos[p.ID] = path
			files = append(files, file{p.File, path})
		}
		for _, a := range t.Attachments {
			if a.File == "" {
				continue
			}
			path := fmt.Sprintf("documents/%d-%d-%s", t.ID, a.ID, a.Filename)
			b.Attachments[a.ID] = path
			files = append(files, file{a.File, path})
		}
	}
	for _, i := range icons {
		path := fmt.Sprintf("icons/%d%s", i.ID, filepath.Ext(i.File))
		b.Icons = append(b.Icons, backupFile{ID: i.ID, Path: path})
		files = append(files, file{i.File, path})
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": exportName(r, s, "zip")}))
	zw := zip.NewWriter(w)
	defer zw.Close()
	jw, err := zw.Create("wherehouse.json")
	if err != nil {
		return
	}
	enc := json.NewEncoder(jw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(b); err != nil {
		return
	}
	for _, f := range files {
		src, err := s.files.Open(f.name)
		if err != nil {
			slog.Warn("export: file missing", "file", f.name)
			continue
		}
		// Photos and PDFs are compressed already.
		dst, err := zw.CreateHeader(&zip.FileHeader{Name: f.path, Method: zip.Store, Modified: time.Now()})
		if err == nil {
			_, err = io.Copy(dst, src)
		}
		src.Close()
		if err != nil {
			slog.Warn("export: writing archive", "err", err)
			return
		}
	}
}
