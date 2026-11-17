package api

import (
	"fmt"
	"maps"
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

type connectedHousehold struct {
	ID        int64
	Name      string
	Members   int
	Connected bool
}

// connectMany connects the app to several households in one go and returns
// its tokens by household.
func connectMany(t *testing.T, h http.Handler, c *http.Cookie, redirect string, hh ...int64) map[int64]string {
	t.Helper()
	ids := make([]string, len(hh))
	for i, id := range hh {
		ids[i] = itoa(id)
	}
	res := decodeAs[map[string]string](t, mustCall(t, h, c, "POST", "/api/connect",
		`{"app":"Warranteed","redirectUri":"`+redirect+`","state":"s","codeChallenge":"`+challengeOf(testVerifier)+
			`","multiple":true,"households":[`+strings.Join(ids, ",")+`]}`, 200))
	u, _ := url.Parse(res["redirect"])
	rec := mustCall(t, h, nil, "POST", "/connect/token", `{"code":"`+u.Query().Get("code")+`","redirect_uri":"`+redirect+
		`","code_verifier":"`+testVerifier+`"}`, 200)
	out := map[int64]string{}
	for _, tok := range decodeAs[struct {
		Tokens []struct {
			Token     string
			Household struct{ ID int64 }
		}
	}](t, rec).Tokens {
		out[tok.Household.ID] = tok.Token
	}
	return out
}

// An app can be connected to several households at once, each with a token
// of its own, and later to more; it sees which it is connected to.
func TestConnectSeveralHouseholds(t *testing.T) {
	h, c := newTestServer(t)
	const redirect = "https://warranty.example.com/wherehouse/callback"
	home := decodeAs[householdsResponse](t, mustCall(t, h, c, "GET", "/api/households", "", 200)).Current
	cottage := decodeAs[struct{ ID int64 }](t, mustCall(t, h, c, "POST", "/api/households", `{"name":"Cottage","currency":"EUR"}`, 201)).ID
	flat := decodeAs[struct{ ID int64 }](t, mustCall(t, h, c, "POST", "/api/households", `{"name":"Flat","currency":"EUR"}`, 201)).ID

	tokens := connectMany(t, h, c, redirect, home, cottage, home)
	if len(tokens) != 2 || tokens[home] == "" || tokens[cottage] == "" || tokens[home] == tokens[cottage] {
		t.Fatalf("tokens = %v", tokens)
	}
	// Each token works in its own household only.
	for hh, tok := range tokens {
		got := decodeAs[struct{ ID int64 }](t, callWithToken(h, "GET", "/api/household", "", tok))
		if got.ID != hh {
			t.Errorf("token for %d works in %d", hh, got.ID)
		}
	}

	listed := func(rec *httptest.ResponseRecorder) map[int64]bool {
		t.Helper()
		if rec.Code != 200 {
			t.Fatalf("households = %d %s", rec.Code, rec.Body)
		}
		out := map[int64]bool{}
		for _, hh := range decodeAs[[]connectedHousehold](t, rec) {
			out[hh.ID] = hh.Connected
		}
		return out
	}
	want := map[int64]bool{home: true, cottage: true, flat: false}
	if got := listed(callWithToken(h, "GET", "/api/connect/households", "", tokens[cottage])); !maps.Equal(got, want) {
		t.Errorf("with the app's token: %v", got)
	}
	if got := listed(mustCall(t, h, c, "GET", "/api/connect/households?app=Warranteed&redirect_uri="+url.QueryEscape(redirect), "", 200)); !maps.Equal(got, want) {
		t.Errorf("signed in: %v", got)
	}
	// Another app, or the same app elsewhere, isn't connected.
	if got := listed(mustCall(t, h, c, "GET", "/api/connect/households?app=Warranteed&redirect_uri="+url.QueryEscape("https://other.example.com/cb"), "", 200)); got[home] {
		t.Errorf("other host: %v", got)
	}
	byHand := decodeAs[struct{ Token string }](t, mustCall(t, h, c, "POST", "/api/tokens", `{"name":"Script"}`, 201)).Token
	if rec := callWithToken(h, "GET", "/api/connect/households", "", byHand); rec.Code != http.StatusForbidden {
		t.Errorf("token made by hand = %d", rec.Code)
	}

	// Connecting one again replaces its token; the others stay.
	again := connectMany(t, h, c, redirect, flat, home)
	if rec := callWithToken(h, "GET", "/api/me", "", tokens[home]); rec.Code != http.StatusUnauthorized {
		t.Errorf("replaced token = %d", rec.Code)
	}
	for _, tok := range []string{again[home], again[flat], tokens[cottage]} {
		if rec := callWithToken(h, "GET", "/api/me", "", tok); rec.Code != 200 {
			t.Errorf("token = %d", rec.Code)
		}
	}

	// Without multiple, the answer keeps its old shape.
	single := connectApp(t, h, c, flat, "https://other.example.com/cb")
	if rec := callWithToken(h, "GET", "/api/me", "", single); rec.Code != 200 {
		t.Errorf("single = %d", rec.Code)
	}
	bad := `{"app":"Warranteed","redirectUri":"` + redirect + `","codeChallenge":"` + challengeOf(testVerifier) + `","multiple":true,"households":%s}`
	mustCall(t, h, c, "POST", "/api/connect", fmt.Sprintf(bad, `[]`), http.StatusBadRequest)
	mustCall(t, h, c, "POST", "/api/connect", fmt.Sprintf(bad, `[999]`), http.StatusNotFound)
}
