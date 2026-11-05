package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestTokensAndLinks(t *testing.T) {
	h, c := newTestServer(t)
	created := decodeAs[struct {
		ID    int64
		Token string
	}](t, mustCall(t, h, c, "POST", "/api/tokens", `{"name":"Warranteed"}`, 201))
	if !strings.HasPrefix(created.Token, store.TokenPrefix) {
		t.Fatalf("token = %q", created.Token)
	}
	withToken := func(method, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://wherehouse.test"+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec := withToken("POST", "/api/things", `{"name":"Dishwasher"}`, created.Token)
	if rec.Code != 201 {
		t.Fatalf("create with token = %d %s", rec.Code, rec.Body)
	}
	it := decodeAs[store.Thing](t, rec)
	if rec := withToken("GET", "/api/things", "", "wherehouse_wrong"); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token = %d", rec.Code)
	}
	// The token works in the household it was made in.
	if things := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/things", "", 200)); len(things) != 1 {
		t.Errorf("things made with the token: %+v", things)
	}
	// Tokens never act as an admin, nor as an owner.
	if rec := withToken("POST", "/api/import/homebox", `{"url":"http://x","token":"y"}`, created.Token); rec.Code != http.StatusForbidden {
		t.Errorf("import with a token = %d", rec.Code)
	}

	if rec := withToken("POST", "/api/household/invites", "", created.Token); rec.Code != http.StatusForbidden {
		t.Errorf("invite with a token = %d", rec.Code)
	}
	// Nor do they manage the account, its settings or its tokens.
	for _, r := range []struct{ method, path, body string }{
		{"GET", "/api/tokens", ""},
		{"POST", "/api/tokens", `{"name":"Another"}`},
		{"DELETE", fmt.Sprintf("/api/tokens/%d", created.ID), ""},
		{"GET", "/api/settings", ""},
		{"PUT", "/api/settings", `{"theme":"dark","notifyDays":"7","paperlessToken":"x"}`},
		{"POST", "/api/settings/test-notification", `{"notifyUrls":"generic://example.com"}`},
		{"POST", "/api/settings/test-paperless", `{"token":"x"}`},
		{"POST", "/api/households", `{"name":"Cottage"}`},
		{"POST", "/api/households/1/switch", ""},
		{"DELETE", "/api/household/members/1", ""},
	} {
		if rec := withToken(r.method, r.path, r.body, created.Token); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s with a token = %d", r.method, r.path, rec.Code)
		}
	}

	link := fmt.Sprintf("/api/things/%d/links", it.ID)
	if rec := withToken("POST", link, `{"url":"https://warranteed.example.com/w/7","title":"Warranty"}`, created.Token); rec.Code != 201 {
		t.Fatalf("link = %d %s", rec.Code, rec.Body)
	}
	// The same address again only updates the title.
	withToken("POST", link, `{"url":"https://warranteed.example.com/w/7","title":"Warranty until 2028"}`, created.Token)
	if rec := withToken("POST", link, `{"url":"javascript:alert(1)"}`, created.Token); rec.Code != 400 {
		t.Errorf("bad link = %d", rec.Code)
	}
	got := decodeAs[thingResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", it.ID), "", 200))
	if len(got.Links) != 1 || got.Links[0].Title != "Warranty until 2028" {
		t.Errorf("links = %+v", got.Links)
	}

	tokens := decodeAs[[]store.Token](t, mustCall(t, h, c, "GET", "/api/tokens", "", 200))
	if len(tokens) != 1 || tokens[0].LastUsedAt == nil {
		t.Errorf("tokens = %+v", tokens)
	}
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/tokens/%d", created.ID), "", 204)
	if rec := withToken("GET", "/api/things", "", created.Token); rec.Code != http.StatusUnauthorized {
		t.Errorf("deleted token = %d", rec.Code)
	}
}
