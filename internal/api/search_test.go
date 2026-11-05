package api

import (
	"fmt"
	"net/url"
	"slices"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestSearch(t *testing.T) {
	h, c := newTestServer(t)
	kitchen := add(t, h, c, fmt.Sprintf(`{"name":"Kuchyň","kindId":%d}`, kindID(t, h, c, "room")))
	drawer := add(t, h, c, fmt.Sprintf(`{"name":"Top drawer","kindId":%d,"parentId":%d}`, kindID(t, h, c, "furniture"), kitchen.ID))
	mixer := add(t, h, c, fmt.Sprintf(`{"name":"Stand mixer","manufacturer":"KitchenAid","parentId":%d}`, kitchen.ID))
	add(t, h, c, fmt.Sprintf(`{"name":"HDMI cable","serial":"XK-200","parentId":%d}`, drawer.ID))

	find := func(q string) []string {
		t.Helper()
		var out []string
		for _, th := range decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/search?q="+url.QueryEscape(q), "", 200)) {
			out = append(out, th.Name)
		}
		return slices.Sorted(slices.Values(out))
	}
	for q, want := range map[string][]string{
		"hdm":          {"HDMI cable"},
		"xk-2":         {"HDMI cable"},
		"kitchenaid":   {"Stand mixer"},
		"kuchyn":       {"HDMI cable", "Kuchyň", "Stand mixer", "Top drawer"},
		"kuchyn mixer": {"Stand mixer"},
		"drawer cable": {"HDMI cable"},
		"nothing here": nil,
		"  ":           nil,
	} {
		if got := find(q); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("search %q = %v, want %v", q, got, want)
		}
	}
	// Renaming a place updates what is found through it.
	mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d", kitchen.ID), fmt.Sprintf(`{"name":"Pantry","kindId":%d}`, kitchen.Kind.ID), 200)
	if got := find("pantry cable"); len(got) != 1 {
		t.Errorf("after rename = %v", got)
	}
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/things/%d", mixer.ID), "", 204)
	if got := find("mixer"); len(got) != 0 {
		t.Errorf("deleted still found = %v", got)
	}
}
