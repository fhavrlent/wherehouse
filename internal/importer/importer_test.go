package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/db"
	"github.com/fhavrlent/wherehouse/internal/homebox"
	"github.com/fhavrlent/wherehouse/internal/store"
)

type dirFiles struct{ dir string }

func (d dirFiles) Save(r io.Reader, ext string) (string, int64, error) {
	f, err := os.CreateTemp(d.dir, "*"+ext)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, r)
	return filepath.Base(f.Name()), n, err
}

func (d dirFiles) Remove(names ...string) {
	for _, n := range names {
		os.Remove(filepath.Join(d.dir, n))
	}
}

// fakeHomeBox serves a small inventory the way HomeBox 0.27 does:
//
//	House
//	  Garage
//	    Toolbox (an item holding another)
//	      Hammer
//	    Drill (sold)
func fakeHomeBox(t *testing.T) *httptest.Server {
	t.Helper()
	var photo bytes.Buffer
	png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	entities := map[string]any{
		"house":  map[string]any{"id": "house", "name": "House", "description": "Our place"},
		"garage": map[string]any{"id": "garage", "name": "Garage", "assetId": "000-007"},
		"toolbox": map[string]any{"id": "toolbox", "name": "Toolbox", "purchasePrice": 40, "serialNumber": "TB-1",
			"entityType": map[string]any{"name": "Tool box", "isLocation": false}},
		"hammer": map[string]any{"id": "hammer", "name": "Hammer", "quantity": 2, "tags": []any{map[string]any{"name": "Tools"}}},
		"drill": map[string]any{
			"id": "drill", "name": "Drill", "assetId": 42, "manufacturer": "Makita", "modelNumber": "DDF484",
			"serialNumber": "SN1", "quantity": 1, "purchaseDate": "2024-03-01T00:00:00Z", "purchasePrice": "129.9",
			"warrantyExpires": "0001-01-01T00:00:00Z", "lifetimeWarranty": false, "insured": true,
			"soldDate": "2026-02-01T00:00:00Z", "soldPrice": 60, "soldTo": "Jan",
			"tags":   []any{map[string]any{"name": "Tools", "color": "#ff0000"}, map[string]any{"name": "Power"}},
			"fields": []any{map[string]any{"type": "text", "name": "Voltage", "textValue": "18 V"}},
			"attachments": []any{
				map[string]any{"id": "a1", "title": "manual.pdf", "type": "manual", "mimeType": "application/pdf"},
				map[string]any{"id": "a2", "title": "drill.png", "type": "photo", "primary": true, "mimeType": "image/png"},
				map[string]any{"id": "a3", "title": "Shop page", "type": "attachment", "sourceType": "external"},
				// Links HomeBox answers with a redirect; neither place is reachable.
				map[string]any{"id": "a4", "title": "receipt.pdf", "type": "receipt"},
				map[string]any{"id": "a5", "title": "Elsewhere", "type": "attachment"},
			},
		},
	}
	tree := []any{map[string]any{"id": "house", "name": "House", "type": "location", "children": []any{
		map[string]any{"id": "garage", "name": "Garage", "type": "location", "children": []any{
			map[string]any{"id": "toolbox", "name": "Toolbox", "type": "item", "children": []any{
				map[string]any{"id": "hammer", "name": "Hammer", "type": "item"},
			}},
			map[string]any{"id": "drill", "name": "Drill", "type": "item"},
		}},
	}}}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/api/v1")
		send := func(v any) { json.NewEncoder(w).Encode(v) }
		switch {
		case p == "/users/self":
			send(map[string]any{"item": map[string]any{"name": "Filip"}})
		case p == "/entities/tree":
			send(tree)
		case p == "/entities/drill/maintenance":
			send([]any{
				map[string]any{"name": "New battery", "scheduledDate": "2027-01-01", "completedDate": "0001-01-01T00:00:00Z"},
				map[string]any{"name": "Cleaned", "completedDate": "2025-05-05T00:00:00Z", "cost": "5"},
			})
		case strings.HasSuffix(p, "/maintenance"):
			send([]any{})
		case p == "/entities/drill/attachments/a1":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("%PDF-1.4"))
		case p == "/entities/drill/attachments/a4":
			http.Redirect(w, r, "https://paperless.invalid/documents/7/details", http.StatusFound)
		case p == "/entities/drill/attachments/a5":
			http.Redirect(w, r, "https://elsewhere.invalid/page", http.StatusFound)
		case p == "/entities/drill/attachments/a2":
			w.Header().Set("Content-Type", "image/png")
			w.Write(photo.Bytes())
		case strings.HasPrefix(p, "/entities/"):
			e, ok := entities[strings.TrimPrefix(p, "/entities/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			send(e)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestImport(t *testing.T) {
	hb := fakeHomeBox(t)
	defer hb.Close()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	st := store.New(database)
	ctx := context.Background()
	u, _ := st.UpsertUser(ctx, store.UpsertUser{Issuer: "test", Subject: "1"})
	hh, err := st.CurrentHousehold(ctx, u.ID, "Home")
	if err != nil {
		t.Fatal(err)
	}

	im := New(st, homebox.New(hb.URL, "key"), dirFiles{dir}, hh, u.ID)
	im.PaperlessURLs = []string{"http://paperless:8000", "https://paperless.invalid"}
	// Before anything goes wrong the problems list is empty, not null, so the page can read it.
	if b, _ := json.Marshal(im.Progress()); !strings.Contains(string(b), `"errors":[]`) {
		t.Fatalf("fresh progress = %s", b)
	}
	if err := im.Run(ctx); err != nil {
		t.Fatal(err)
	}
	p := im.Progress()
	if p.Things != 5 || p.Photos != 1 || p.Documents != 2 || p.Maintenance != 2 || len(p.Errors) != 0 {
		t.Fatalf("progress = %+v", p)
	}

	kind := func(id string) store.KindRef {
		t.Helper()
		th, err := st.FromHomeBox(ctx, hh, id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		return th.Kind
	}
	if kind("house").Key != "building" || kind("garage").Key != "room" || kind("hammer").Key != "thing" {
		t.Errorf("kinds: house %+v, garage %+v, hammer %+v", kind("house"), kind("garage"), kind("hammer"))
	}
	// A box that is an item keeps its details, and its HomeBox type becomes a kind.
	toolbox, _ := st.FromHomeBox(ctx, hh, "toolbox")
	if toolbox.Kind.Name != "Tool box" || toolbox.Serial != "TB-1" || toolbox.PurchasePrice == nil || *toolbox.PurchasePrice != 40 {
		t.Errorf("toolbox = %+v", toolbox)
	}

	drill, _ := st.FromHomeBox(ctx, hh, "drill")
	if drill.Label != "000-042" || drill.Manufacturer != "Makita" || len(drill.Tags) != 2 || drill.Cover == nil {
		t.Errorf("drill = %+v", drill)
	}
	if drill.PurchaseDate == nil || *drill.PurchaseDate != "2024-03-01" || *drill.PurchasePrice != 129.9 || drill.WarrantyUntil != nil ||
		!drill.Insured || drill.SoldDate == nil || *drill.SoldPrice != 60 || drill.SoldTo != "Jan" {
		t.Errorf("drill purchase = %+v", drill.Purchase)
	}
	if len(drill.Fields) != 1 || drill.Fields[0].Value != "18 V" {
		t.Errorf("drill fields = %+v", drill.Fields)
	}
	if len(drill.Attachments) != 2 || len(drill.Maintenance) != 2 {
		t.Errorf("drill documents %+v, maintenance %+v", drill.Attachments, drill.Maintenance)
	}
	for _, a := range drill.Attachments {
		if a.Storage == store.StoragePaperless && (a.Kind != "receipt" || a.PaperlessDocID == nil || *a.PaperlessDocID != 7 ||
			a.MimeType != "application/pdf") {
			t.Errorf("paperless link = %+v", a)
		}
		if a.Storage == store.StorageLocal && a.Kind != "manual" {
			t.Errorf("local document = %+v", a)
		}
	}
	hammer, _ := st.FromHomeBox(ctx, hh, "hammer")
	if hammer.Quantity != 2 || len(hammer.Path) != 3 || hammer.Path[2].Name != "Toolbox" {
		t.Errorf("hammer = %+v", hammer)
	}
	garage, _ := st.FromHomeBox(ctx, hh, "garage")
	if garage.Label != "000-007" {
		t.Errorf("garage label = %q", garage.Label)
	}

	again := New(st, homebox.New(hb.URL, "key"), dirFiles{dir}, hh, u.ID)
	if err := again.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if p := again.Progress(); p.Things != 0 || p.Skipped != 5 {
		t.Errorf("second import = %+v", p)
	}
	if res, _ := st.Search(ctx, hh, "makita", false); len(res) != 1 {
		t.Errorf("search after import = %+v", res)
	}
	bad := New(st, homebox.New(hb.URL, "wrong"), dirFiles{dir}, hh, u.ID)
	if err := bad.Run(ctx); err == nil || bad.Progress().Error == "" {
		t.Errorf("import with a wrong key = %v", err)
	}
}
