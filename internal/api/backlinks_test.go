package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fhavrlent/wherehouse/internal/config"
	"github.com/fhavrlent/wherehouse/internal/paperless"
	"github.com/fhavrlent/wherehouse/internal/store"
)

// fakePaperless keeps documents' custom field values, with field 3 being
// the Wherehouse URL field and field 4 another one.
type fakePaperless struct {
	mu     sync.Mutex
	fields map[int64][]paperless.CustomField
}

func (f *fakePaperless) value(doc int64) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.fields[doc] {
		if c.Field == 3 {
			v, _ := c.Value.(string)
			return v, true
		}
	}
	return "", false
}

func (f *fakePaperless) server(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var doc int64
		fmt.Sscanf(r.URL.Path, "/api/documents/%d/", &doc)
		if doc != 0 && r.Header.Get("Authorization") == "Token nope" {
			http.Error(w, `{"detail":"Not found."}`, http.StatusNotFound)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.URL.Path == "/api/custom_fields/":
			w.Write([]byte(`{"results":[{"id":3,"name":"Wherehouse","data_type":"url"}]}`))
		case doc != 0 && r.Method == "PATCH":
			var in struct {
				CustomFields []paperless.CustomField `json:"custom_fields"`
			}
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &in)
			f.fields[doc] = in.CustomFields
			w.Write([]byte(`{}`))
		case doc != 0 && r.URL.Query().Get("fields") == "custom_fields":
			json.NewEncoder(w).Encode(map[string]any{"custom_fields": f.fields[doc]})
		case doc != 0:
			fmt.Fprintf(w, `{"id":%d,"title":"Receipt %d","original_file_name":"r.pdf","mime_type":"application/pdf","created":"2026-01-01"}`, doc, doc)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// eventually waits for background link updates.
func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting: %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBacklinksFollowThings(t *testing.T) {
	pl := &fakePaperless{fields: map[int64][]paperless.CustomField{}}
	srv := pl.server(t)
	e := newTestEnv(t, func(c *config.Config) {
		c.Paperless = config.Paperless{URL: srv.URL, Field: "Wherehouse"}
	})
	c := e.signIn(t, "a")
	h := e.h
	mustCall(t, h, c, "PUT", "/api/settings", `{"theme":"system","notifyDays":"7","paperlessToken":"tok","paperlessBacklinks":true}`, 200)
	tv := add(t, h, c, `{"name":"TV"}`)
	radio := add(t, h, c, `{"name":"Radio"}`)
	link := func(id int64) {
		mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/attachments/paperless", id), `{"documentIds":[7],"kind":"receipt"}`, 201)
	}
	is := func(want string) func() bool {
		return func() bool { v, _ := pl.value(7); return v == want }
	}

	// One thing: the link opens it.
	link(tv.ID)
	eventually(t, "link to the TV", is(fmt.Sprintf("http://wherehouse.test/i/%d", tv.ID)))
	// Two: the list of them.
	link(radio.ID)
	eventually(t, "link to the list", is("http://wherehouse.test/d/7"))
	// Back to one when the document is taken off a thing, and none when the
	// thing goes.
	radio = decodeAs[store.Thing](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", radio.ID), "", 200))
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/attachments/%d", radio.Attachments[0].ID), "", 204)
	eventually(t, "link back to the TV", is(fmt.Sprintf("http://wherehouse.test/i/%d", tv.ID)))
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/things/%d", tv.ID), "", 204)
	eventually(t, "link removed", func() bool { _, ok := pl.value(7); return !ok })
}

func TestBacklinksFilledInLater(t *testing.T) {
	pl := &fakePaperless{fields: map[int64][]paperless.CustomField{
		// Someone typed their own link into document 9.
		9: {{Field: 3, Value: "https://example.com/mine"}, {Field: 4, Value: "kept"}},
		// Document 8 has the link of the previous version.
		8: {{Field: 3, Value: "http://wherehouse.test/d/8"}},
	}}
	srv := pl.server(t)
	e := newTestEnv(t, func(c *config.Config) {
		c.Paperless = config.Paperless{URL: srv.URL, Field: "Wherehouse"}
	})
	c := e.signIn(t, "a")
	h := e.h
	mustCall(t, h, c, "PUT", "/api/settings", `{"theme":"system","notifyDays":"7","paperlessToken":"tok"}`, 200)
	tv := add(t, h, c, `{"name":"TV"}`)
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/attachments/paperless", tv.ID), `{"documentIds":[7,8,9],"kind":"receipt"}`, 201)
	time.Sleep(100 * time.Millisecond)
	if v, ok := pl.value(7); ok {
		t.Fatalf("link written with links off: %q", v)
	}

	mustCall(t, h, c, "PUT", "/api/settings", `{"theme":"system","notifyDays":"7","paperlessBacklinks":true}`, 200)
	var res struct{ Documents, Failed int }
	json.Unmarshal(mustCall(t, h, c, "POST", "/api/paperless/links", "", 200).Body.Bytes(), &res)
	if res.Documents != 3 || res.Failed != 0 {
		t.Errorf("filled in = %+v", res)
	}
	want := fmt.Sprintf("http://wherehouse.test/i/%d", tv.ID)
	for _, doc := range []int64{7, 8} {
		if v, _ := pl.value(doc); v != want {
			t.Errorf("document %d links to %q", doc, v)
		}
	}
	if v, _ := pl.value(9); v != "https://example.com/mine" || len(pl.fields[9]) != 2 {
		t.Errorf("someone's own link changed: %+v", pl.fields[9])
	}
}

// Someone whose Paperless can't reach the document deletes its thing: the
// link is removed with the connection that wrote it.
func TestBacklinkTriesTheNextWriter(t *testing.T) {
	pl := &fakePaperless{fields: map[int64][]paperless.CustomField{}}
	srv := pl.server(t)
	e := newTestEnv(t, func(c *config.Config) {
		c.Paperless = config.Paperless{URL: srv.URL, Field: "Wherehouse"}
	})
	alice, bob := e.signIn(t, "alice"), e.signIn(t, "bob")
	h := e.h
	mustCall(t, h, alice, "PUT", "/api/settings", `{"theme":"system","notifyDays":"7","paperlessToken":"tok","paperlessBacklinks":true}`, 200)
	mustCall(t, h, bob, "PUT", "/api/settings", `{"theme":"system","notifyDays":"7","paperlessToken":"nope","paperlessBacklinks":true}`, 200)
	invite := decodeAs[map[string]string](t, mustCall(t, h, alice, "POST", "/api/household/invites", "", 201))
	mustCall(t, h, bob, "POST", "/api/invites/"+invite["url"][strings.LastIndex(invite["url"], "/")+1:]+"/accept", "", 204)

	tv := add(t, h, alice, `{"name":"TV"}`)
	mustCall(t, h, alice, "POST", fmt.Sprintf("/api/things/%d/attachments/paperless", tv.ID), `{"documentIds":[7],"kind":"receipt"}`, 201)
	eventually(t, "link written", func() bool { _, ok := pl.value(7); return ok })
	mustCall(t, h, bob, "DELETE", fmt.Sprintf("/api/things/%d", tv.ID), "", 204)
	eventually(t, "link removed", func() bool { _, ok := pl.value(7); return !ok })
}

// A backup names documents by ID without anyone having reached them here:
// restoring it never borrows the connection of someone who linked them.
func TestRestoreDoesNotBorrowConnections(t *testing.T) {
	pl := &fakePaperless{fields: map[int64][]paperless.CustomField{}}
	srv := pl.server(t)
	e := newTestEnv(t, func(c *config.Config) {
		c.Paperless = config.Paperless{URL: srv.URL, Field: "Wherehouse"}
	})
	alice, carol := e.signIn(t, "alice"), e.signIn(t, "carol")
	h := e.h
	mustCall(t, h, alice, "PUT", "/api/settings", `{"theme":"system","notifyDays":"7","paperlessToken":"tok","paperlessBacklinks":true}`, 200)
	mustCall(t, h, carol, "PUT", "/api/settings", `{"theme":"system","notifyDays":"7","paperlessToken":"nope","paperlessBacklinks":true}`, 200)
	tv := add(t, h, alice, `{"name":"TV"}`)
	mustCall(t, h, alice, "POST", fmt.Sprintf("/api/things/%d/attachments/paperless", tv.ID), `{"documentIds":[7],"kind":"receipt"}`, 201)
	want := fmt.Sprintf("http://wherehouse.test/i/%d", tv.ID)
	eventually(t, "link written", func() bool { v, _ := pl.value(7); return v == want })

	backup := mustCall(t, h, alice, "GET", "/api/export.zip", "", 200).Body.Bytes()
	restoreBackup(t, h, carol, backup, 201)
	time.Sleep(300 * time.Millisecond)
	if v, _ := pl.value(7); v != want {
		t.Errorf("link changed to %q through someone else's connection", v)
	}
}

func TestThingOfAnotherHousehold(t *testing.T) {
	e := newTestEnv(t)
	c := e.signIn(t, "a")
	h := e.h
	tv := add(t, h, c, `{"name":"TV"}`)
	first := households(t, h, c).Current
	cottage := decodeAs[struct{ ID int64 }](t, mustCall(t, h, c, "POST", "/api/households", `{"name":"Cottage","currency":"EUR"}`, 201))
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/households/%d/switch", cottage.ID), "", 204)
	mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", tv.ID), "", 404)
	got := decodeAs[struct {
		ID   int64
		Name string
	}](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d/household", tv.ID), "", 200))
	if got.ID != first {
		t.Errorf("household = %+v, want %d", got, first)
	}
	// Someone else learns nothing.
	other := e.signIn(t, "b")
	mustCall(t, h, other, "GET", fmt.Sprintf("/api/things/%d/household", tv.ID), "", 404)
}
