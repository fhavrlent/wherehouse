package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestReport(t *testing.T) {
	h, c := newTestServer(t)
	room := kindID(t, h, c, "room")
	office := add(t, h, c, fmt.Sprintf(`{"name":"Office","kindId":%d}`, room))
	kitchen := add(t, h, c, fmt.Sprintf(`{"name":"Kitchen","kindId":%d}`, room))
	soon := time.Now().AddDate(0, 0, 30).Format(store.DateLayout)
	add(t, h, c, fmt.Sprintf(`{"name":"Laptop","parentId":%d,"purchasePrice":1000,"insured":true,"tags":["Tech"],"warrantyUntil":%q}`, office.ID, soon))
	add(t, h, c, fmt.Sprintf(`{"name":"Cables","parentId":%d,"purchasePrice":5,"quantity":4,"tags":["Tech"]}`, office.ID))
	add(t, h, c, fmt.Sprintf(`{"name":"Mixer","parentId":%d,"purchasePrice":5000,"currency":"CZK"}`, kitchen.ID))
	add(t, h, c, fmt.Sprintf(`{"name":"Spoon","parentId":%d}`, kitchen.ID))
	add(t, h, c, fmt.Sprintf(`{"name":"Old TV","purchasePrice":300,"soldDate":"%d-01-02","soldPrice":50}`, time.Now().Year()))

	r := decodeAs[store.Report](t, mustCall(t, h, c, "GET", "/api/report", "", 200))
	if r.Currency != "EUR" || r.Value["EUR"] != 1020 || r.Value["CZK"] != 5000 || r.Insured["EUR"] != 1000 {
		t.Errorf("value = %v, insured = %v", r.Value, r.Insured)
	}
	if r.Things != 4 || r.Priced != 3 || r.Sold["EUR"] != 50 {
		t.Errorf("things %d, priced %d, sold %v", r.Things, r.Priced, r.Sold)
	}
	if len(r.ByPlace) != 2 || r.ByPlace[0].Name != "Office" || r.ByPlace[0].Value["EUR"] != 1020 {
		t.Errorf("by place = %+v", r.ByPlace)
	}
	if len(r.ByTag) != 1 || r.ByTag[0].Count != 2 || len(r.WarrantyEnding) != 1 {
		t.Errorf("by tag = %+v, warranty ending = %+v", r.ByTag, r.WarrantyEnding)
	}
	// Inside a place, only what is in it counts.
	in := decodeAs[store.Report](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/report?in=%d", kitchen.ID), "", 200))
	if in.Value["EUR"] != 0 || in.Value["CZK"] != 5000 || in.Things != 2 {
		t.Errorf("kitchen report = %+v", in)
	}
	if insured := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/report/insured", "", 200)); len(insured) != 1 {
		t.Errorf("insured = %+v", insured)
	}
}
