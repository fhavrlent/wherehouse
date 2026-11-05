package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHooks(t *testing.T) {
	var mu sync.Mutex
	var pings []string
	var sigs []string
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		pings = append(pings, string(b))
		sigs = append(sigs, r.Header.Get("X-Wherehouse-Signature"))
		mu.Unlock()
	}))
	defer app.Close()

	e := newTestEnv(t)
	c := e.signIn(t, "a")
	h := e.h
	hh := decodeAs[householdsResponse](t, mustCall(t, h, c, "GET", "/api/households", "", 200)).Current

	// Connect an app whose callback is on the test server.
	redirect := app.URL + "/wherehouse/callback"
	res := decodeAs[map[string]string](t, mustCall(t, h, c, "POST", "/api/connect",
		`{"app":"Warranteed","redirectUri":"`+redirect+`","state":"s","household":`+itoa(hh)+`}`, 200))
	u, _ := url.Parse(res["redirect"])
	rec := call(h, "POST", "/connect/token", `{"code":"`+u.Query().Get("code")+`","redirect_uri":"`+redirect+`"}`, nil)
	token := decodeAs[struct{ Token string }](t, rec).Token
	withToken := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://wherehouse.test"+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		h.ServeHTTP(out, req)
		return out
	}

	// Only the approved host, and only with an app's token.
	if rec := withToken("PUT", "/api/hook", `{"url":"http://169.254.169.254/latest"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("hook elsewhere = %d", rec.Code)
	}
	mustCall(t, h, c, "PUT", "/api/hook", `{"url":"`+app.URL+`/hooks/wherehouse"}`, http.StatusForbidden)
	manual := decodeAs[createdToken](t, mustCall(t, h, c, "POST", "/api/tokens", `{"name":"Mine"}`, 201)).Secret
	req := httptest.NewRequest("PUT", "http://wherehouse.test/api/hook", strings.NewReader(`{"url":"`+app.URL+`/x"}`))
	req.Header.Set("Authorization", "Bearer "+manual)
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	if out.Code != http.StatusForbidden {
		t.Errorf("hook with a token made by hand = %d", out.Code)
	}

	rec = withToken("PUT", "/api/hook", `{"url":"`+app.URL+`/hooks/wherehouse"}`)
	if rec.Code != 200 {
		t.Fatalf("set hook = %d %s", rec.Code, rec.Body)
	}
	secret := decodeAs[map[string]string](t, rec)["secret"]

	// The app's own change doesn't ping it; a burst of others' changes pings once.
	withToken("POST", "/api/things", `{"name":"From the app"}`)
	add(t, h, c, `{"name":"One"}`)
	add(t, h, c, `{"name":"Two"}`)
	time.Sleep(hookDelay + time.Second)
	mu.Lock()
	defer mu.Unlock()
	if len(pings) != 1 {
		t.Fatalf("pings = %q", pings)
	}
	if !strings.Contains(pings[0], `"event":"changed"`) || sigs[0] != Sign(secret, []byte(pings[0])) {
		t.Errorf("ping %q signed %q", pings[0], sigs[0])
	}
}
