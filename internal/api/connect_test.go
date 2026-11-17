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
	request := func(state string) string {
		return `{"app":"Warranteed","redirectUri":"` + redirect + `","state":"` + state + `","codeChallenge":"` +
			challengeOf(testVerifier) + `","household":` + itoa(hh) + `}`
	}
	// Without a PKCE challenge, there is no code.
	mustCall(t, h, c, "POST", "/api/connect", `{"app":"Warranteed","redirectUri":"`+redirect+`","state":"s0","household":`+itoa(hh)+`}`, 400)
	res := allow(request("s1"))
	u, _ := url.Parse(res["redirect"])
	if !strings.HasPrefix(res["redirect"], redirect+"?") || u.Query().Get("state") != "s1" || u.Query().Get("code") == "" {
		t.Fatalf("redirect = %q", res["redirect"])
	}
	code := u.Query().Get("code")

	swap := func(code, redirect, verifier string) *httptest.ResponseRecorder {
		return call(h, "POST", "/connect/token", `{"code":"`+code+`","redirect_uri":"`+redirect+`","code_verifier":"`+verifier+`"}`, nil)
	}
	// Another redirect address doesn't get it, and uses the code up.
	if rec := swap(code, "https://evil.example.com/cb", testVerifier); rec.Code != http.StatusBadRequest {
		t.Errorf("wrong redirect = %d", rec.Code)
	}
	if rec := swap(code, redirect, testVerifier); rec.Code != http.StatusBadRequest {
		t.Errorf("used code = %d", rec.Code)
	}
	// Nor does someone with the code but not the verifier.
	u1, _ := url.Parse(allow(request("s1b"))["redirect"])
	if rec := swap(u1.Query().Get("code"), redirect, strings.Repeat("x", 43)); rec.Code != http.StatusBadRequest {
		t.Errorf("wrong verifier = %d", rec.Code)
	}

	u2, _ := url.Parse(allow(request("s2"))["redirect"])
	rec := swap(u2.Query().Get("code"), redirect, testVerifier)
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
	mustCall(t, h, c, "POST", "/api/connect", `{"app":"X","redirectUri":"`+redirect+`","codeChallenge":"`+challengeOf(testVerifier)+`","household":999}`, 404)
	mustCall(t, h, c, "POST", "/api/connect", `{"app":"X","redirectUri":"javascript:alert(1)","household":`+itoa(hh)+`}`, 400)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// testVerifier is the PKCE verifier tests connect apps with.
var testVerifier = strings.Repeat("v", 50)

// connectApp connects an app the way one does, and returns its token.
func connectApp(t *testing.T, h http.Handler, c *http.Cookie, hh int64, redirect string) string {
	t.Helper()
	res := decodeAs[map[string]string](t, mustCall(t, h, c, "POST", "/api/connect",
		`{"app":"Warranteed","redirectUri":"`+redirect+`","state":"s","codeChallenge":"`+challengeOf(testVerifier)+
			`","household":`+itoa(hh)+`}`, 200))
	u, _ := url.Parse(res["redirect"])
	rec := mustCall(t, h, nil, "POST", "/connect/token", `{"code":"`+u.Query().Get("code")+`","redirect_uri":"`+redirect+
		`","code_verifier":"`+testVerifier+`"}`, 200)
	return decodeAs[struct{ Token string }](t, rec).Token
}
