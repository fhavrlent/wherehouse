package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestHouseholds(t *testing.T) {
	e := newTestEnv(t)
	alice, bob := e.signIn(t, "alice"), e.signIn(t, "bob")

	// Everyone starts in a household of their own, and sees only theirs.
	add(t, e.h, alice, `{"name":"Alice's bike"}`)
	if things := decodeAs[[]store.Thing](t, mustCall(t, e.h, bob, "GET", "/api/things", "", 200)); len(things) != 0 {
		t.Fatalf("bob sees %+v", things)
	}
	bike := decodeAs[[]store.Thing](t, mustCall(t, e.h, alice, "GET", "/api/things", "", 200))[0]
	mustCall(t, e.h, bob, "GET", fmt.Sprintf("/api/things/%d", bike.ID), "", 404)
	mustCall(t, e.h, bob, "PUT", fmt.Sprintf("/api/things/%d", bike.ID), `{"name":"Mine now"}`, 404)
	if res := decodeAs[[]store.Thing](t, mustCall(t, e.h, bob, "GET", "/api/search?q=bike", "", 200)); len(res) != 0 {
		t.Errorf("bob finds %+v", res)
	}

	// Alice invites Bob. His own household, still empty, goes away.
	invite := decodeAs[map[string]string](t, mustCall(t, e.h, alice, "POST", "/api/household/invites", "", 201))
	token := invite["url"][strings.LastIndex(invite["url"], "/")+1:]
	info := decodeAs[map[string]any](t, mustCall(t, e.h, bob, "GET", "/api/invites/"+token, "", 200))
	if info["member"] != false {
		t.Errorf("invite info = %v", info)
	}
	// Members can't invite or rename.
	mustCall(t, e.h, bob, "POST", "/api/invites/"+token+"/accept", "", 204)
	mustCall(t, e.h, bob, "POST", "/api/household/invites", "", 403)
	mustCall(t, e.h, bob, "PUT", "/api/household", `{"name":"Bob's","currency":"EUR"}`, 403)
	list := decodeAs[householdsResponse](t, mustCall(t, e.h, bob, "GET", "/api/households", "", 200))
	if len(list.Households) != 1 || list.Households[0].Members != 2 {
		t.Fatalf("bob's households = %+v", list)
	}
	if got := decodeAs[[]store.Thing](t, mustCall(t, e.h, bob, "GET", "/api/things", "", 200)); len(got) != 1 {
		t.Errorf("bob sees %d things after joining", len(got))
	}

	// A second household of Alice's is separate, and she switches between them.
	shared := list.Current
	mustCall(t, e.h, alice, "POST", "/api/households", `{"name":"Cottage","currency":"CZK"}`, 201)
	if got := decodeAs[[]store.Thing](t, mustCall(t, e.h, alice, "GET", "/api/things", "", 200)); len(got) != 0 {
		t.Errorf("new household has %d things", len(got))
	}
	cur := decodeAs[householdResponse](t, mustCall(t, e.h, alice, "GET", "/api/household", "", 200))
	if cur.Name != "Cottage" || cur.Currency != "CZK" || cur.Role != store.RoleOwner {
		t.Errorf("cottage = %+v", cur)
	}
	mustCall(t, e.h, alice, "POST", fmt.Sprintf("/api/households/%d/switch", shared), "", 204)
	mustCall(t, e.h, bob, "POST", fmt.Sprintf("/api/households/%d/switch", cur.ID), "", 404)

	// The last owner can't leave while others are in; Bob can.
	members := decodeAs[householdResponse](t, mustCall(t, e.h, alice, "GET", "/api/household", "", 200)).Members
	var aliceID, bobID int64
	for _, m := range members {
		if m.Name == "alice" {
			aliceID = m.UserID
		} else {
			bobID = m.UserID
		}
	}
	mustCall(t, e.h, alice, "DELETE", fmt.Sprintf("/api/household/members/%d", aliceID), "", 400)
	mustCall(t, e.h, bob, "DELETE", fmt.Sprintf("/api/household/members/%d", aliceID), "", 403)
	mustCall(t, e.h, bob, "DELETE", fmt.Sprintf("/api/household/members/%d", bobID), "", 204)
	if got := decodeAs[[]store.Thing](t, mustCall(t, e.h, bob, "GET", "/api/things", "", 200)); len(got) != 0 {
		t.Errorf("bob still sees %d things after leaving", len(got))
	}
	mustCall(t, e.h, alice, "GET", "/api/invites/nope", "", 404)
}

func TestKindsAndIcons(t *testing.T) {
	h, c := newTestServer(t)
	kinds := decodeAs[[]store.Kind](t, mustCall(t, h, c, "GET", "/api/kinds", "", 200))
	if len(kinds) != 5 {
		t.Fatalf("built-in kinds = %+v", kinds)
	}
	tool := decodeAs[store.Kind](t, mustCall(t, h, c, "POST", "/api/kinds", `{"name":"Power tool","icon":"drill"}`, 201))
	mustCall(t, h, c, "POST", "/api/kinds", `{"name":"X","icon":"<script>"}`, 400)
	mustCall(t, h, c, "POST", "/api/kinds", `{"name":"X","icon":"custom:999"}`, 400)

	rec := upload(t, h, c, "/api/icons", map[string][]byte{"tool.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/></svg>`)})
	if rec.Code != 201 {
		t.Fatalf("icon upload = %d %s", rec.Code, rec.Body)
	}
	icon := decodeAs[store.Icon](t, rec)
	served := mustCall(t, h, c, "GET", fmt.Sprintf("/api/icons/%d", icon.ID), "", 200)
	if served.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(served.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("icon headers = %v", served.Header())
	}
	if rec := upload(t, h, c, "/api/icons", map[string][]byte{"x.txt": []byte("hello")}); rec.Code != 400 {
		t.Errorf("text icon = %d", rec.Code)
	}

	custom := fmt.Sprintf("custom:%d", icon.ID)
	mustCall(t, h, c, "PUT", fmt.Sprintf("/api/kinds/%d", tool.ID), fmt.Sprintf(`{"name":"Power tool","icon":%q}`, custom), 200)
	drill := add(t, h, c, fmt.Sprintf(`{"name":"Drill","kindId":%d}`, tool.ID))
	if drill.Kind.Icon != custom || drill.Kind.Name != "Power tool" {
		t.Errorf("drill kind = %+v", drill.Kind)
	}
	// Kinds and icons in use stay; the built-in "thing" always does.
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/icons/%d", icon.ID), "", 400)
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/kinds/%d", tool.ID), "", 400)
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/kinds/%d", kindID(t, h, c, "thing")), "", 400)
	// The kind's name is searchable.
	if res := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/search?q=power", "", 200)); len(res) != 1 {
		t.Errorf("search by kind = %+v", res)
	}
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/things/%d", drill.ID), "", 204)
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/kinds/%d", tool.ID), "", 204)
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/icons/%d", icon.ID), "", 204)
}
