package api

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/fhavrlent/wherehouse/internal/store"
)

// A connected app's token does what the app needs, nothing more; a token
// made by hand does everything. An app can end its own token.
func TestAppTokenScope(t *testing.T) {
	h, c := newTestServer(t)
	hh := decodeAs[householdsResponse](t, mustCall(t, h, c, "GET", "/api/households", "", 200)).Current
	app := connectApp(t, h, c, hh, "https://warranty.example.com/wherehouse/callback")
	byHand := decodeAs[struct{ Token string }](t, mustCall(t, h, c, "POST", "/api/tokens", `{"name":"Script"}`, 201)).Token
	thing := add(t, h, c, `{"name":"Drill"}`)

	for _, r := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/api/things", "", 200},
		{"GET", "/api/household", "", 200},
		{"PATCH", fmt.Sprintf("/api/things/%d", thing.ID), `{"quantity":2}`, 200},
		{"PUT", fmt.Sprintf("/api/things/%d", thing.ID), `{"name":"Drill"}`, http.StatusForbidden},
		{"GET", "/api/export.csv", "", http.StatusForbidden},
		{"GET", "/api/export.zip", "", http.StatusForbidden},
		{"POST", "/api/kinds", `{"name":"Tool"}`, http.StatusForbidden},
		{"POST", "/api/household/invites", `{}`, http.StatusForbidden},
	} {
		if rec := callWithToken(h, r.method, r.path, r.body, app); rec.Code != r.want {
			t.Errorf("app %s %s = %d, want %d: %s", r.method, r.path, rec.Code, r.want, rec.Body)
		}
	}
	if rec := callWithToken(h, "GET", "/api/export.csv", "", byHand); rec.Code != 200 {
		t.Errorf("by hand export = %d", rec.Code)
	}

	// Disconnecting ends the app's token.
	if rec := callWithToken(h, "DELETE", "/api/token", "", app); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d %s", rec.Code, rec.Body)
	}
	if rec := callWithToken(h, "GET", "/api/things", "", app); rec.Code != http.StatusUnauthorized {
		t.Errorf("after revoking = %d", rec.Code)
	}
	if rec := callWithToken(h, "GET", "/api/things", "", byHand); rec.Code != 200 {
		t.Errorf("other token after revoking = %d", rec.Code)
	}
	mustCall(t, h, c, "DELETE", "/api/token", "", http.StatusForbidden)
}

// PATCH changes only what it is sent: a move, a sale or insurance set
// meanwhile isn't written back over.
func TestPatchThing(t *testing.T) {
	h, c := newTestServer(t)
	box := add(t, h, c, `{"name":"Box"}`)
	drill := add(t, h, c, fmt.Sprintf(`{"name":"Drill","parentId":%d,"serial":"S1","tags":["Tools"],"purchasePrice":99}`, box.ID))
	path := fmt.Sprintf("/api/things/%d", drill.ID)
	// Changed here meanwhile.
	mustCall(t, h, c, "PUT", path, fmt.Sprintf(`{"name":"Drill","parentId":%d,"serial":"S1","purchasePrice":99,"insured":true,
		"soldDate":"2026-11-01","soldPrice":50}`, box.ID), 200)

	got := decodeAs[thingResponse](t, mustCall(t, h, c, "PATCH", path, `{"quantity":3,"serial":"S2"}`, 200))
	if got.Quantity != 3 || got.Serial != "S2" || got.ParentID == nil || *got.ParentID != box.ID || !got.Insured ||
		got.SoldDate == nil || *got.SoldPrice != 50 || *got.PurchasePrice != 99 || got.Name != "Drill" || len(got.Tags) != 1 {
		t.Errorf("after patching = %+v", got)
	}
	mustCall(t, h, c, "PATCH", path, `{"quantity":-1}`, http.StatusBadRequest)
	mustCall(t, h, c, "PATCH", path, `[1]`, http.StatusBadRequest)
	mustCall(t, h, c, "PATCH", "/api/things/999", `{"quantity":2}`, http.StatusNotFound)
}

// A thing is linked to one thing of an app; moving a link lets the purchase
// details of the thing it was on go back to this app.
func TestAppLinkTaken(t *testing.T) {
	h, c := newTestServer(t)
	tv := add(t, h, c, `{"name":"TV"}`)
	radio := add(t, h, c, `{"name":"Radio"}`)
	link := func(id int64, body string, want int) {
		t.Helper()
		mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d/apps/warranteed", id), body, want)
	}
	link(tv.ID, `{"externalId":"1","purchase":{"purchasePrice":100}}`, 200)
	link(tv.ID, `{"externalId":"1","purchase":{"purchasePrice":120}}`, 200) // the same link changes
	link(tv.ID, `{"externalId":"2"}`, http.StatusConflict)

	link(radio.ID, `{"externalId":"1","purchase":{"purchasePrice":30}}`, 200)
	edited := decodeAs[thingResponse](t, mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d", tv.ID),
		`{"name":"TV","purchasePrice":5}`, 200))
	if edited.PurchaseOwner != "" || *edited.PurchasePrice != 5 {
		t.Errorf("tv after the link moved = owner %q, price %v", edited.PurchaseOwner, *edited.PurchasePrice)
	}
	link(tv.ID, `{"externalId":"2"}`, 200)
}

// The change feed shows a thing whose label changed, and things whose path
// changed because what they are in was moved or renamed.
func TestChangesLabelsAndPaths(t *testing.T) {
	h, c := newTestServer(t)
	shed := add(t, h, c, `{"name":"Shed"}`)
	box := add(t, h, c, fmt.Sprintf(`{"name":"Box","parentId":%d}`, shed.ID))
	drill := add(t, h, c, fmt.Sprintf(`{"name":"Drill","parentId":%d}`, box.ID))
	other := add(t, h, c, `{"name":"Garage"}`)

	changed := func(change func()) []int64 {
		t.Helper()
		// Times are to the second: what changed before is a second behind.
		time.Sleep(1100 * time.Millisecond)
		until := decodeAs[store.Changes](t, mustCall(t, h, c, "GET", "/api/changes", "", 200)).Until
		change()
		ch := decodeAs[store.Changes](t, mustCall(t, h, c, "GET", "/api/changes?since="+url.QueryEscape(until), "", 200))
		var ids []int64
		for _, th := range ch.Things {
			ids = append(ids, th.ID)
		}
		return ids
	}
	same := func(got []int64, want ...int64) bool {
		if len(got) != len(want) {
			return false
		}
		in := map[int64]bool{}
		for _, id := range got {
			in[id] = true
		}
		for _, id := range want {
			if !in[id] {
				return false
			}
		}
		return true
	}

	var code string
	if got := changed(func() {
		code = decodeAs[map[string]string](t, mustCall(t, h, c, "POST", "/api/labels", fmt.Sprintf(`{"thingId":%d}`, drill.ID), 200))["code"]
	}); !same(got, drill.ID) {
		t.Errorf("label attached: changed %v", got)
	}
	if got := changed(func() { mustCall(t, h, c, "DELETE", "/api/labels/"+code, "", 204) }); !same(got, drill.ID) {
		t.Errorf("label detached: changed %v", got)
	}
	if got := changed(func() {
		mustCall(t, h, c, "POST", "/api/move", fmt.Sprintf(`{"into":%d,"things":[%d]}`, other.ID, box.ID), 204)
	}); !same(got, box.ID, drill.ID) {
		t.Errorf("box moved: changed %v", got)
	}
	if got := changed(func() {
		mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d", other.ID), `{"name":"Big garage"}`, 200)
	}); !same(got, other.ID, box.ID, drill.ID) {
		t.Errorf("garage renamed: changed %v", got)
	}
	// Changing something else of it leaves what is inside alone.
	if got := changed(func() {
		mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d", other.ID), `{"name":"Big garage","notes":"Cold"}`, 200)
	}); !same(got, other.ID) {
		t.Errorf("garage notes: changed %v", got)
	}
}
