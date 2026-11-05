package api

import (
	"fmt"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestThingsHoldThings(t *testing.T) {
	h, c := newTestServer(t)
	building, room, box := kindID(t, h, c, "building"), kindID(t, h, c, "room"), kindID(t, h, c, "box")
	house := add(t, h, c, fmt.Sprintf(`{"name":"House","kindId":%d}`, building))
	office := add(t, h, c, fmt.Sprintf(`{"name":"Office","kindId":%d,"parentId":%d}`, room, house.ID))
	cables := add(t, h, c, fmt.Sprintf(`{"name":"Cable box","kindId":%d,"parentId":%d,"purchasePrice":12}`, box, office.ID))
	hdmi := add(t, h, c, fmt.Sprintf(`{"name":" HDMI cable ","quantity":3,"parentId":%d}`, cables.ID))
	if hdmi.Name != "HDMI cable" || hdmi.Kind.Key != "thing" || len(hdmi.Path) != 3 || hdmi.Path[2].Name != "Cable box" {
		t.Errorf("hdmi = %+v", hdmi)
	}
	// A thing that isn't a place kind can hold things too, and then shows
	// as a place: a drill case with the bits in it.
	drill := add(t, h, c, fmt.Sprintf(`{"name":"Drill case","parentId":%d,"serial":"D1"}`, office.ID))
	add(t, h, c, fmt.Sprintf(`{"name":"Drill bits","parentId":%d}`, drill.ID))
	places := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/things?places=1", "", 200))
	names := map[string]bool{}
	for _, p := range places {
		names[p.Name] = true
	}
	if !names["Drill case"] || !names["Cable box"] || names["HDMI cable"] {
		t.Errorf("places = %v", names)
	}

	got := decodeAs[thingResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", office.ID), "", 200))
	if len(got.Contents) != 2 || got.Contains != 2 {
		t.Errorf("office = %+v", got)
	}
	inside := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things?in=%d&inside=1", house.ID), "", 200))
	if len(inside) != 5 {
		t.Errorf("inside the house: %d things", len(inside))
	}

	// Nothing goes inside itself or what is inside it.
	mustCall(t, h, c, "POST", "/api/move", fmt.Sprintf(`{"into":%d,"things":[%d]}`, hdmi.ID, house.ID), 400)
	mustCall(t, h, c, "POST", "/api/move", fmt.Sprintf(`{"into":%d,"things":[%d]}`, house.ID, house.ID), 400)
	for _, body := range []string{`{"name":"  "}`, `{"name":"X","kindId":9999}`, `{"name":"X","parentId":9999}`,
		`{"name":"X","quantity":-1}`, `{"name":"X","purchaseDate":"soon"}`, `{"name":"X","currency":"euro"}`} {
		mustCall(t, h, c, "POST", "/api/things", body, 400)
	}

	// Moving the box takes what's in it along.
	mustCall(t, h, c, "POST", "/api/move", fmt.Sprintf(`{"into":null,"things":[%d]}`, cables.ID), 204)
	moved := decodeAs[thingResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", hdmi.ID), "", 200))
	if len(moved.Path) != 1 || moved.Path[0].ID != cables.ID {
		t.Errorf("hdmi after moving its box: %+v", moved.Path)
	}
	home := decodeAs[homeResponse](t, mustCall(t, h, c, "GET", "/api/home", "", 200))
	if len(home.Places) != 2 || len(home.Recent) == 0 {
		t.Errorf("home = %+v", home)
	}
	for _, r := range home.Recent {
		if r.IsPlace() {
			t.Errorf("a place among recent things: %s", r.Name)
		}
	}

	// Only things holding nothing can be deleted.
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/things/%d", cables.ID), "", 400)
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/things/%d", hdmi.ID), "", 204)
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/things/%d", cables.ID), "", 204)
}

func TestPurchaseDetails(t *testing.T) {
	h, c := newTestServer(t)
	tv := add(t, h, c, `{"name":"TV","purchaseDate":"2025-01-10","purchasePrice":899.9,"purchaseFrom":"Alza","currency":"czk",
		"warrantyUntil":"2027-01-10","insured":true}`)
	if tv.PurchasePrice == nil || *tv.PurchasePrice != 899.9 || tv.Currency != "CZK" || !tv.Insured || *tv.WarrantyUntil != "2027-01-10" {
		t.Errorf("tv = %+v", tv.Purchase)
	}
	sold := decodeAs[store.Thing](t, mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d", tv.ID),
		`{"name":"TV","purchasePrice":899.9,"soldDate":"2026-06-01","soldPrice":300,"soldTo":"Neighbour"}`, 200))
	if sold.SoldDate == nil || *sold.SoldPrice != 300 || sold.PurchaseFrom != "" || sold.Insured {
		t.Errorf("after selling = %+v", sold.Purchase)
	}
	// Sale details are searchable.
	if res := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/search?q=neighbour", "", 200)); len(res) != 1 {
		t.Errorf("search sold to = %+v", res)
	}
}

func TestPhotos(t *testing.T) {
	h, c := newTestServer(t)
	it := add(t, h, c, `{"name":"Drill"}`)
	rec := upload(t, h, c, fmt.Sprintf("/api/things/%d/photos", it.ID), map[string][]byte{"a.png": pngBytes(t), "b.png": pngBytes(t)})
	if rec.Code != 201 {
		t.Fatalf("upload = %d %s", rec.Code, rec.Body)
	}
	if rec := upload(t, h, c, fmt.Sprintf("/api/things/%d/photos", it.ID), map[string][]byte{"a.txt": []byte("hello")}); rec.Code != 400 {
		t.Errorf("text as photo = %d", rec.Code)
	}
	got := decodeAs[thingResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", it.ID), "", 200))
	if got.Cover == nil || len(got.Photos) != 2 {
		t.Fatalf("thing = %+v", got)
	}
	second := got.Photos[1].ID
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/photos/%d/cover", second), "", 204)
	if got := decodeAs[thingResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", it.ID), "", 200)); *got.Cover != second {
		t.Errorf("cover = %d, want %d", *got.Cover, second)
	}
	if ct := mustCall(t, h, c, "GET", fmt.Sprintf("/api/photos/%d?size=thumb", second), "", 200).Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("thumb type = %q", ct)
	}
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/photos/%d", second), "", 204)
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/things/%d", it.ID), "", 204)
	mustCall(t, h, c, "GET", fmt.Sprintf("/api/photos/%d", got.Photos[0].ID), "", 404)
}

func TestArchive(t *testing.T) {
	h, c := newTestServer(t)
	room := add(t, h, c, fmt.Sprintf(`{"name":"Office","kindId":%d}`, kindID(t, h, c, "room")))
	lamp := add(t, h, c, fmt.Sprintf(`{"name":"Lamp","parentId":%d}`, room.ID))
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/archive", lamp.ID), "", 200)
	p := decodeAs[thingResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", room.ID), "", 200))
	if p.Contains != 0 || len(p.Contents) != 0 {
		t.Errorf("room after archiving = %+v", p)
	}
	if archived := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/things?archived=1", "", 200)); len(archived) != 1 {
		t.Errorf("archived = %+v", archived)
	}
	if res := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/search?q=lamp", "", 200)); len(res) != 0 {
		t.Errorf("archived found: %+v", res)
	}
}
