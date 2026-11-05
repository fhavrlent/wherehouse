package api

import (
	"fmt"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestCopyAndTemplates(t *testing.T) {
	h, c := newTestServer(t)
	shelf := add(t, h, c, fmt.Sprintf(`{"name":"Shelf","kindId":%d}`, kindID(t, h, c, "furniture")))
	other := add(t, h, c, fmt.Sprintf(`{"name":"Other shelf","kindId":%d}`, kindID(t, h, c, "furniture")))
	bulb := add(t, h, c, fmt.Sprintf(`{"name":"LED bulb E27","parentId":%d,"manufacturer":"IKEA","purchasePrice":3,"tags":["Lights"],
		"soldDate":"2026-01-01"}`, shelf.ID))
	upload(t, h, c, fmt.Sprintf("/api/things/%d/photos", bulb.ID), map[string][]byte{"a.png": pngBytes(t)})
	mustCall(t, h, c, "POST", "/api/labels", fmt.Sprintf(`{"thingId":%d}`, bulb.ID), 200)

	cp := decodeAs[store.Thing](t, mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/copy", bulb.ID), `{"sameParent":true,"photos":true}`, 201))
	if cp.Name != bulb.Name || cp.ParentID == nil || *cp.ParentID != shelf.ID || cp.Manufacturer != "IKEA" ||
		len(cp.Tags) != 1 || cp.Cover == nil || cp.Label != "" || cp.SoldDate != nil {
		t.Errorf("copy = %+v", cp)
	}

	tpl := decodeAs[store.Thing](t, mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/copy", bulb.ID), `{"template":true,"name":"Bulb"}`, 201))
	if !tpl.Template || tpl.ParentID != nil || tpl.Name != "Bulb" {
		t.Errorf("template = %+v", tpl)
	}
	// Templates are listed only as templates.
	if list := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/things?templates=1", "", 200)); len(list) != 1 {
		t.Errorf("templates = %+v", list)
	}
	if res := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/search?q=bulb", "", 200)); len(res) != 2 {
		t.Errorf("search finds %d (the template shouldn't count)", len(res))
	}
	made := decodeAs[store.Thing](t, mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/copy", tpl.ID),
		fmt.Sprintf(`{"name":"Hall bulb","parentId":%d}`, other.ID), 201))
	if made.Template || *made.ParentID != other.ID || made.Name != "Hall bulb" || made.PurchasePrice == nil {
		t.Errorf("from template = %+v", made)
	}
}
