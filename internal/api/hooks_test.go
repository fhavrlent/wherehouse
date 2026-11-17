package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fhavrlent/wherehouse/internal/config"
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

	// The test app is on the local network: the admin allowed its host.
	e := newTestEnv(t, func(cfg *config.Config) { cfg.HookAllowedHosts = []string{"127.0.0.1"} })
	c := e.signIn(t, "a")
	h := e.h
	hh := decodeAs[householdsResponse](t, mustCall(t, h, c, "GET", "/api/households", "", 200)).Current

	// Connect an app whose callback is on the test server.
	token := connectApp(t, h, c, hh, app.URL+"/wherehouse/callback")
	var rec *httptest.ResponseRecorder
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

// A connected app on the local network isn't pinged unless the admin
// allowed its host: connecting can't make the server post into its network.
func TestHooksOnlyToAllowedLocalHosts(t *testing.T) {
	var mu sync.Mutex
	pings := 0
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pings++
		mu.Unlock()
	}))
	defer app.Close()

	e := newTestEnv(t)
	c := e.signIn(t, "a")
	hh := decodeAs[householdsResponse](t, mustCall(t, e.h, c, "GET", "/api/households", "", 200)).Current
	token := connectApp(t, e.h, c, hh, app.URL+"/wherehouse/callback")
	if rec := callWithToken(e.h, "PUT", "/api/hook", `{"url":"`+app.URL+`/hooks/wherehouse"}`, token); rec.Code != 200 {
		t.Fatalf("set hook = %d %s", rec.Code, rec.Body)
	}
	add(t, e.h, c, `{"name":"One"}`)
	time.Sleep(hookDelay + time.Second)
	mu.Lock()
	defer mu.Unlock()
	if pings != 0 {
		t.Errorf("pinged %d times", pings)
	}
}

// Making or joining a household pings the person's connected apps, whatever
// household they are connected to, so they can offer to connect it too.
func TestHooksNewHousehold(t *testing.T) {
	var mu sync.Mutex
	var pings []map[string]any
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		json.NewDecoder(r.Body).Decode(&p)
		mu.Lock()
		pings = append(pings, p)
		mu.Unlock()
	}))
	defer app.Close()
	e := newTestEnv(t, func(cfg *config.Config) { cfg.HookAllowedHosts = []string{"127.0.0.1"} })
	a, b := e.signIn(t, "a"), e.signIn(t, "b")
	home := decodeAs[householdsResponse](t, mustCall(t, e.h, a, "GET", "/api/households", "", 200)).Current
	token := connectApp(t, e.h, a, home, app.URL+"/wherehouse/callback")
	if rec := callWithToken(e.h, "PUT", "/api/hook", `{"url":"`+app.URL+`/hooks/wherehouse"}`, token); rec.Code != 200 {
		t.Fatalf("hook = %d %s", rec.Code, rec.Body)
	}
	// Joining deletes an empty household of one's own, with its tokens.
	add(t, e.h, a, `{"name":"Keeps home"}`)
	// Someone else's household isn't announced to a's apps.
	mustCall(t, e.h, b, "POST", "/api/households", `{"name":"B's","currency":"EUR"}`, 201)
	cottage := decodeAs[struct{ ID int64 }](t, mustCall(t, e.h, a, "POST", "/api/households", `{"name":"Cottage","currency":"EUR"}`, 201)).ID
	bHome := decodeAs[householdsResponse](t, mustCall(t, e.h, b, "GET", "/api/households", "", 200)).Current
	invite := decodeAs[map[string]string](t, mustCall(t, e.h, b, "POST", "/api/household/invites", `{}`, 201))["url"]
	code := invite[strings.LastIndex(invite, "/")+1:]
	mustCall(t, e.h, a, "POST", "/api/invites/"+code+"/accept", "", 204)
	mustCall(t, e.h, a, "POST", "/api/invites/"+code+"/accept", "", 204) // already in: no ping
	time.Sleep(time.Second)

	mu.Lock()
	defer mu.Unlock()
	var got []int64
	for _, p := range pings {
		if p["event"] != "households" {
			t.Errorf("ping = %v", p)
			continue
		}
		got = append(got, int64(p["household"].(float64)))
	}
	if len(got) != 2 || got[0] != cottage || got[1] != bHome {
		t.Errorf("households announced = %v, want [%d %d]", got, cottage, bHome)
	}
}
