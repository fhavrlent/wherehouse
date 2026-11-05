package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestConnect(t *testing.T) {
	e := newTestEnv(t)
	c := e.signIn(t, "a")
	h := e.h
	hh := decodeAs[householdsResponse](t, mustCall(t, h, c, "GET", "/api/households", "", 200)).Current
	add(t, h, c, `{"name":"Mine"}`)

	allow := func(body string) map[string]string {
		return decodeAs[map[string]string](t, mustCall(t, h, c, "POST", "/api/connect", body, 200))
	}
	redirect := "https://warranty.example.com/wherehouse/callback"
	res := allow(`{"app":"Warranteed","redirectUri":"` + redirect + `","state":"s1","household":` + itoa(hh) + `}`)
	u, _ := url.Parse(res["redirect"])
	if !strings.HasPrefix(res["redirect"], redirect+"?") || u.Query().Get("state") != "s1" || u.Query().Get("code") == "" {
		t.Fatalf("redirect = %q", res["redirect"])
	}
	code := u.Query().Get("code")

	swap := func(code, redirect string) *httptest.ResponseRecorder {
		return call(h, "POST", "/connect/token", `{"code":"`+code+`","redirect_uri":"`+redirect+`"}`, nil)
	}
	// Another redirect address doesn't get it, and uses the code up.
	if rec := swap(code, "https://evil.example.com/cb"); rec.Code != http.StatusBadRequest {
		t.Errorf("wrong redirect = %d", rec.Code)
	}
	if rec := swap(code, redirect); rec.Code != http.StatusBadRequest {
		t.Errorf("used code = %d", rec.Code)
	}

	u2, _ := url.Parse(allow(`{"app":"Warranteed","redirectUri":"` + redirect + `","state":"s2","household":` + itoa(hh) + `}`)["redirect"])
	rec := swap(u2.Query().Get("code"), redirect)
	if rec.Code != 200 {
		t.Fatalf("swap = %d %s", rec.Code, rec.Body)
	}
	got := decodeAs[struct {
		Token     string
		Household struct{ Name string }
	}](t, rec)
	req := httptest.NewRequest("GET", "http://wherehouse.test/api/things", nil)
	req.Header.Set("Authorization", "Bearer "+got.Token)
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	if out.Code != 200 || len(decodeAs[[]store.Thing](t, out)) != 1 {
		t.Errorf("token works = %d", out.Code)
	}

	// Only households the person is in, and only web addresses.
	mustCall(t, h, c, "POST", "/api/connect", `{"app":"X","redirectUri":"`+redirect+`","household":999}`, 404)
	mustCall(t, h, c, "POST", "/api/connect", `{"app":"X","redirectUri":"javascript:alert(1)","household":`+itoa(hh)+`}`, 400)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
