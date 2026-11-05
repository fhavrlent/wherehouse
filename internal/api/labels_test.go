package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestLabels(t *testing.T) {
	e := newTestEnv(t)
	c := e.signIn(t, "a")
	h := e.h
	box := add(t, h, c, fmt.Sprintf(`{"name":"Box","kindId":%d}`, kindID(t, h, c, "box")))
	it := add(t, h, c, `{"name":"Drill"}`)

	type codeResp struct{ Code string }
	first := decodeAs[codeResp](t, mustCall(t, h, c, "POST", "/api/labels", fmt.Sprintf(`{"thingId":%d}`, box.ID), 200))
	again := decodeAs[codeResp](t, mustCall(t, h, c, "POST", "/api/labels", fmt.Sprintf(`{"thingId":%d}`, box.ID), 200))
	if len(first.Code) != store.CodeLength || first.Code != again.Code {
		t.Fatalf("codes %q, %q", first.Code, again.Code)
	}
	got := decodeAs[labelResponse](t, mustCall(t, h, c, "GET", "/api/labels/"+strings.ToLower(first.Code), "", 200))
	if got.Thing == nil || got.Thing.ID != box.ID {
		t.Errorf("label = %+v", got)
	}
	if res := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/search?q="+first.Code, "", 200)); len(res) != 1 {
		t.Errorf("search by code = %+v", res)
	}

	blank := decodeAs[map[string][]string](t, mustCall(t, h, c, "POST", "/api/labels/blank", `{"count":3}`, 201))["codes"]
	if l := decodeAs[labelResponse](t, mustCall(t, h, c, "GET", "/api/labels/"+blank[0], "", 200)); l.Thing != nil {
		t.Errorf("blank label = %+v", l)
	}
	mustCall(t, h, c, "POST", "/api/labels", fmt.Sprintf(`{"code":%q,"thingId":%d}`, blank[0], it.ID), 200)
	mustCall(t, h, c, "POST", "/api/labels", fmt.Sprintf(`{"code":%q,"thingId":%d}`, first.Code, it.ID), 400)
	mustCall(t, h, c, "POST", "/api/labels", fmt.Sprintf(`{"code":%q,"thingId":%d}`, blank[1], it.ID), 200)
	if l := decodeAs[labelResponse](t, mustCall(t, h, c, "GET", "/api/labels/"+blank[0], "", 200)); l.Thing != nil {
		t.Errorf("old label still on %+v", l.Thing)
	}
	mustCall(t, h, c, "POST", "/api/labels", fmt.Sprintf(`{"code":"000-042","thingId":%d}`, it.ID), 200)
	mustCall(t, h, c, "POST", "/api/labels", fmt.Sprintf(`{"code":"bad code!","thingId":%d}`, it.ID), 400)
	mustCall(t, h, c, "DELETE", "/api/labels/000-042", "", 204)
	mustCall(t, h, c, "GET", "/api/labels/NOPE9", "", 404)
	mustCall(t, h, c, "POST", "/api/labels/blank", `{"count":0}`, 400)

	// Someone else's label is unknown to them, and can't be taken over.
	other := e.signIn(t, "b")
	theirs := add(t, h, other, `{"name":"Theirs"}`)
	mustCall(t, h, other, "GET", "/api/labels/"+first.Code, "", 404)
	mustCall(t, h, other, "POST", "/api/labels", fmt.Sprintf(`{"code":%q,"thingId":%d}`, blank[2], theirs.ID), 400)
}
