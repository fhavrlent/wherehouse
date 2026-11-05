package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/store"
)

// fullHousehold fills the current household with one of everything a
// backup holds, and returns its export.
func fullHousehold(t *testing.T, h http.Handler, c *http.Cookie) []byte {
	t.Helper()
	mustCall(t, h, c, "POST", "/api/fields", `{"name":"Voltage","type":"text"}`, 201)
	icon := decodeAs[store.Icon](t, upload(t, h, c, "/api/icons", map[string][]byte{"drill.png": pngBytes(t)}))
	kind := decodeAs[store.Kind](t, mustCall(t, h, c, "POST", "/api/kinds", fmt.Sprintf(`{"name":"Power tool","icon":"custom:%d"}`, icon.ID), 201))
	room := add(t, h, c, fmt.Sprintf(`{"name":"Garage","kindId":%d}`, kindID(t, h, c, "room")))
	box := add(t, h, c, fmt.Sprintf(`{"name":"Box","kindId":%d,"parentId":%d}`, kindID(t, h, c, "box"), room.ID))
	drill := add(t, h, c, fmt.Sprintf(`{"name":"Drill","kindId":%d,"parentId":%d,"quantity":2,"serial":"SN1","purchasePrice":99.5,"tags":["Tools"],"fields":{"1":"18 V"}}`,
		kind.ID, box.ID))
	tags := decodeAs[[]store.Tag](t, mustCall(t, h, c, "GET", "/api/tags", "", 200))
	mustCall(t, h, c, "PATCH", fmt.Sprintf("/api/tags/%d", tags[0].ID), `{"name":"Tools","color":"violet"}`, 204)
	upload(t, h, c, fmt.Sprintf("/api/things/%d/photos", drill.ID), map[string][]byte{"a.png": pngBytes(t)})
	second := decodeAs[[]store.Photo](t, upload(t, h, c, fmt.Sprintf("/api/things/%d/photos", drill.ID), map[string][]byte{"b.jpg": []byte("\xff\xd8\xff second")}))
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/photos/%d/cover", second[0].ID), "", 204)
	upload(t, h, c, fmt.Sprintf("/api/things/%d/attachments", drill.ID), map[string][]byte{"manual.pdf": []byte("%PDF-1.4 manual")})
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/maintenance", drill.ID), `{"title":"New brushes","doneOn":"2026-03-01","cost":12}`, 201)
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/links", drill.ID), `{"url":"https://example.com/drill","title":"Maker"}`, 201)
	mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d/apps/warranteed", drill.ID),
		`{"externalId":"7","url":"https://warranty.example/w/7","title":"Warranty","purchase":{"purchaseDate":"2026-01-10","purchasePrice":120,"currency":"CZK"}}`, 200)
	mustCall(t, h, c, "POST", "/api/labels", fmt.Sprintf(`{"thingId":%d,"code":"ABC-123"}`, drill.ID), 200)
	old := add(t, h, c, `{"name":"Old saw"}`)
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/archive", old.ID), "", 200)
	return mustCall(t, h, c, "GET", "/api/export.zip", "", 200).Body.Bytes()
}

func restoreBackup(t *testing.T, h http.Handler, c *http.Cookie, zipData []byte, want int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "http://wherehouse.test/api/restore", bytes.NewReader(zipData))
	req.Header.Set("Content-Type", "application/zip")
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("restore = %d, want %d: %s", rec.Code, want, rec.Body)
	}
	return rec
}

// readBackup reads the wherehouse.json of an export and its files.
func readBackup(t *testing.T, data []byte) (backup, map[string][]byte) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var b backup
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		body, _ := io.ReadAll(rc)
		rc.Close()
		if f.Name == "wherehouse.json" {
			if err := json.Unmarshal(body, &b); err != nil {
				t.Fatal(err)
			}
		} else {
			files[f.Name] = body
		}
	}
	return b, files
}

// sameThing is what of a thing must survive a restore, without IDs.
type sameThing struct {
	Name, Kind, Path, Label, Tags, Fields, Links, Maintenance, PurchaseOwner, Purchase string
	Photos, Documents                                                                  []string
	Quantity                                                                           int
	ArchivedAt                                                                         *string
	CreatedAt                                                                          string
}

func comparable(t *testing.T, data []byte) []sameThing {
	t.Helper()
	b, files := readBackup(t, data)
	kinds := map[int64]string{}
	for _, k := range b.Kinds {
		kinds[k.ID] = k.Key + "/" + k.Name + "/" + fmt.Sprint(k.IsPlace)
		if k.Icon != "" && k.Key == "" {
			// An uploaded icon is compared by its picture.
			kinds[k.ID] += "/" + k.Icon[:min(len(k.Icon), 7)]
		}
	}
	colors := map[string]string{}
	for _, tag := range b.Tags {
		colors[tag.Name] = tag.Color
	}
	var out []sameThing
	for _, th := range b.Things {
		s := sameThing{Name: th.Name, Kind: kinds[th.Kind.ID], Label: th.Label, PurchaseOwner: th.PurchaseOwner,
			Quantity: th.Quantity, ArchivedAt: th.ArchivedAt, CreatedAt: th.CreatedAt}
		for _, p := range th.Path {
			s.Path += p.Name + "/"
		}
		for _, tag := range th.Tags {
			s.Tags += tag.Name + ":" + colors[tag.Name] + ","
		}
		for _, f := range th.Fields {
			s.Fields += f.Name + "=" + f.Value + ","
		}
		for _, l := range th.Links {
			s.Links += l.Source + " " + l.ExternalID + " " + l.URL + " " + l.Title + ","
		}
		for _, m := range th.Maintenance {
			s.Maintenance += fmt.Sprintf("%s %v %v %s,", m.Title, *m.DoneOn, *m.Cost, m.CreatedAt)
		}
		p, _ := json.Marshal(th.Purchase)
		s.Purchase = string(p)
		// Photos in order, cover first.
		for _, ph := range th.Photos {
			s.Photos = append(s.Photos, string(files[b.Photos[ph.ID]]))
		}
		for _, a := range th.Attachments {
			s.Documents = append(s.Documents, a.Filename+" "+string(files[b.Attachments[a.ID]]))
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b sameThing) int {
		if a.Path+a.Name < b.Path+b.Name {
			return -1
		}
		return 1
	})
	return out
}

func TestRestore(t *testing.T) {
	h, c := newTestServer(t)
	data := withOldTimes(t, fullHousehold(t, h, c))

	// Into a Wherehouse set up afresh, everything comes back.
	h2, c2 := newTestServer(t)
	res := decodeAs[restoreResult](t, restoreBackup(t, h2, c2, data, 201))
	if res.Things != 4 || res.Photos != 2 || res.Documents != 1 || res.LabelsSkipped != 0 {
		t.Errorf("result = %+v", res)
	}
	// It's the household now worked in.
	again := mustCall(t, h2, c2, "GET", "/api/export.zip", "", 200).Body.Bytes()
	want, got := comparable(t, data), comparable(t, again)
	if len(want) != 4 {
		t.Fatalf("export has %d things", len(want))
	}
	for i := range want {
		w, _ := json.Marshal(want[i])
		g, _ := json.Marshal(got[i])
		if string(w) != string(g) {
			t.Errorf("thing %d:\nwant %s\ngot  %s", i, w, g)
		}
	}
	b, _ := readBackup(t, again)
	if len(b.Fields) != 1 || len(b.Icons) != 1 || b.Household.Name != res.Name {
		t.Errorf("restored household = %+v, fields %+v, icons %+v", b.Household, b.Fields, b.Icons)
	}
	// The app's link finds the restored thing.
	rec := mustCall(t, h2, c2, "GET", "/api/apps/warranteed/7", "", 200)
	if th := decodeAs[store.Thing](t, rec); th.Name != "Drill" || th.PurchaseOwner != "warranteed" {
		t.Errorf("thing by app link = %+v", th)
	}

	// Beside the household it was made of, labels on things there stay there.
	res = decodeAs[restoreResult](t, restoreBackup(t, h, c, data, 201))
	if res.Things != 4 || res.LabelsSkipped != 1 {
		t.Errorf("result beside the original = %+v", res)
	}
	if hs := households(t, h, c); len(hs.Households) != 2 || hs.Current != res.HouseholdID {
		t.Errorf("households = %+v", hs)
	}
}

func TestRestoreRefusesWhatIsNotABackup(t *testing.T) {
	h, c := newTestServer(t)
	before := households(t, h, c)
	restoreBackup(t, h, c, []byte("not a zip"), 400)

	// A backup that breaks halfway leaves nothing behind.
	data := fullHousehold(t, h, c)
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if f.Name == "wherehouse.json" || len(f.Name) > 7 && f.Name[:7] == "photos/" {
			continue // no photos
		}
		w, _ := zw.Create(f.Name)
		rc, _ := f.Open()
		io.Copy(w, rc)
		rc.Close()
	}
	b, _ := readBackup(t, data)
	w, _ := zw.Create("wherehouse.json")
	json.NewEncoder(w).Encode(b)
	zw.Close()
	restoreBackup(t, h, c, buf.Bytes(), 400)
	after := households(t, h, c)
	if len(after.Households) != len(before.Households) || after.Current != before.Current {
		t.Errorf("households after a failed restore = %+v", after)
	}
}

func households(t *testing.T, h http.Handler, c *http.Cookie) householdsResponse {
	t.Helper()
	return decodeAs[householdsResponse](t, mustCall(t, h, c, "GET", "/api/households", "", 200))
}

// withOldTimes gives every thing and maintenance entry in a backup a time
// of its own in the past, so a restore that didn't keep them shows.
func withOldTimes(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	b, _ := readBackup(t, data)
	for i := range b.Things {
		b.Things[i].CreatedAt = fmt.Sprintf("2020-01-%02dT10:00:00Z", i+1)
		b.Things[i].UpdatedAt = fmt.Sprintf("2021-01-%02dT10:00:00Z", i+1)
		for j := range b.Things[i].Maintenance {
			b.Things[i].Maintenance[j].CreatedAt = "2022-02-02T10:00:00Z"
		}
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		w, _ := zw.Create(f.Name)
		if f.Name == "wherehouse.json" {
			json.NewEncoder(w).Encode(b)
			continue
		}
		rc, _ := f.Open()
		io.Copy(w, rc)
		rc.Close()
	}
	zw.Close()
	return buf.Bytes()
}
