package api

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestAppLinks(t *testing.T) {
	h, c := newTestServer(t)
	start := time.Now().Add(-time.Second).UTC().Format(time.RFC3339)
	tv := add(t, h, c, `{"name":"TV","purchasePrice":10}`)
	path := fmt.Sprintf("/api/things/%d/apps/warranteed", tv.ID)

	got := decodeAs[thingResponse](t, mustCall(t, h, c, "PUT", path, `{"externalId":"42","url":"https://w.example.com/w/42",
		"title":"Warranty","data":{"until":"2027-01-01"},
		"purchase":{"purchaseDate":"2025-01-01","purchasePrice":899,"purchaseFrom":"Alza","warrantyUntil":"2027-01-01"}}`, 200))
	if got.PurchaseOwner != "warranteed" || *got.PurchasePrice != 899 || len(got.Links) != 1 || got.Links[0].Source != "warranteed" {
		t.Fatalf("linked = %+v", got)
	}
	// Owned details can't be changed here; insurance and a sale can.
	edited := decodeAs[store.Thing](t, mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d", tv.ID),
		`{"name":"TV","purchasePrice":1,"warrantyUntil":"2099-01-01","insured":true}`, 200))
	if *edited.PurchasePrice != 899 || *edited.WarrantyUntil != "2027-01-01" || !edited.Insured {
		t.Errorf("after editing here = %+v", edited.Purchase)
	}
	found := decodeAs[store.Thing](t, mustCall(t, h, c, "GET", "/api/apps/warranteed/42", "", 200))
	if found.ID != tv.ID {
		t.Errorf("lookup = %+v", found)
	}
	mustCall(t, h, c, "GET", "/api/apps/warranteed/43", "", 404)
	mustCall(t, h, c, "PUT", path, `{"externalId":""}`, 400)
	mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d/apps/BAD", tv.ID), `{"externalId":"1"}`, 400)

	// Linking the same warranty to another thing moves the link.
	radio := add(t, h, c, `{"name":"Radio"}`)
	mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d/apps/warranteed", radio.ID), `{"externalId":"42"}`, 200)
	if found := decodeAs[store.Thing](t, mustCall(t, h, c, "GET", "/api/apps/warranteed/42", "", 200)); found.ID != radio.ID {
		t.Errorf("lookup after moving = %d", found.ID)
	}
	// The TV keeps its details, now its own again once unlinked.
	if tvNow := decodeAs[thingResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", tv.ID), "", 200)); len(tvNow.Links) != 0 {
		t.Errorf("tv links = %+v", tvNow.Links)
	}
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/things/%d/apps/warranteed", tv.ID), "", 404)

	// The change feed has what changed, and what was deleted.
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/things/%d", radio.ID), "", 204)
	ch := decodeAs[store.Changes](t, mustCall(t, h, c, "GET", "/api/changes?since="+url.QueryEscape(start), "", 200))
	if len(ch.Things) != 1 || ch.Things[0].ID != tv.ID || len(ch.Deleted) != 1 || ch.Deleted[0] != radio.ID || ch.Until == "" {
		t.Errorf("changes = %+v", ch)
	}
	mustCall(t, h, c, "GET", "/api/changes?since=yesterday", "", 400)
	mustCall(t, h, c, "GET", "/api/homebox/nope", "", 404)
}

// Apps following the change feed see a document deleted or changed on a
// thing, as they see a photo deleted.
func TestChangesHaveDocuments(t *testing.T) {
	h, c := newTestServer(t)
	thing := add(t, h, c, `{"name":"Drill"}`)
	doc := decodeAs[store.Attachment](t, upload(t, h, c, fmt.Sprintf("/api/things/%d/attachments", thing.ID),
		map[string][]byte{"manual.pdf": []byte("%PDF-1.4 manual")}))
	for _, change := range []func(){
		func() {
			mustCall(t, h, c, "PATCH", fmt.Sprintf("/api/attachments/%d", doc.ID), `{"kind":"warranty","filename":"card.pdf"}`, 200)
		},
		func() { mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/attachments/%d", doc.ID), "", 204) },
	} {
		until := decodeAs[store.Changes](t, mustCall(t, h, c, "GET", "/api/changes", "", 200)).Until
		time.Sleep(1100 * time.Millisecond) // times are to the second
		change()
		ch := decodeAs[store.Changes](t, mustCall(t, h, c, "GET", "/api/changes?since="+url.QueryEscape(until), "", 200))
		if len(ch.Things) != 1 || ch.Things[0].ID != thing.ID {
			t.Errorf("changes = %+v", ch)
		}
	}
}

// A maintenance entry added, changed or deleted shows as a change to its
// thing, so apps that keep their own entries there see it.
func TestChangesHaveMaintenance(t *testing.T) {
	h, c := newTestServer(t)
	thing := add(t, h, c, `{"name":"Drill"}`)
	var entry int64
	for _, change := range []func(){
		func() {
			m := decodeAs[store.Maintenance](t, mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/maintenance", thing.ID),
				`{"title":"Brushes","doneOn":"2026-03-01"}`, 201))
			entry = m.ID
		},
		func() {
			mustCall(t, h, c, "PUT", fmt.Sprintf("/api/maintenance/%d", entry), `{"title":"New brushes","doneOn":"2026-03-02"}`, 200)
		},
		func() { mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/maintenance/%d", entry), "", 204) },
	} {
		until := decodeAs[store.Changes](t, mustCall(t, h, c, "GET", "/api/changes", "", 200)).Until
		time.Sleep(1100 * time.Millisecond) // times are to the second
		change()
		ch := decodeAs[store.Changes](t, mustCall(t, h, c, "GET", "/api/changes?since="+url.QueryEscape(until), "", 200))
		if len(ch.Things) != 1 || ch.Things[0].ID != thing.ID {
			t.Errorf("changes = %+v", ch)
		}
	}
}
