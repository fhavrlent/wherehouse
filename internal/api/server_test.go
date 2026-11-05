package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/fhavrlent/wherehouse/internal/auth"
	"github.com/fhavrlent/wherehouse/internal/config"
	"github.com/fhavrlent/wherehouse/internal/db"
	"github.com/fhavrlent/wherehouse/internal/secret"
	"github.com/fhavrlent/wherehouse/internal/store"
)

// testEnv is a server and its store, for tests that need more than one
// person.
type testEnv struct {
	h  http.Handler
	st *store.Store
}

// signIn adds a person and returns their session cookie.
func (e *testEnv) signIn(t *testing.T, subject string) *http.Cookie {
	t.Helper()
	ctx := context.Background()
	u, err := e.st.UpsertUser(ctx, store.UpsertUser{Issuer: "test", Subject: subject, Email: subject + "@example.com", Name: subject})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.st.CreateSession(ctx, "token-"+subject, u.ID, "", "", time.Hour); err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "wherehouse_session", Value: "token-" + subject}
}

// newTestServer returns a server with one signed-in user and that user's
// session cookie.
func newTestServer(t *testing.T) (http.Handler, *http.Cookie) {
	t.Helper()
	e := newTestEnv(t)
	return e.h, e.signIn(t, "a")
}

func newTestEnv(t *testing.T, configure ...func(*config.Config)) *testEnv {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "wherehouse.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	cfg := &config.Config{
		AppURL:     &url.URL{Scheme: "http", Host: "wherehouse.test"},
		DataDir:    dir,
		SessionTTL: time.Hour,
		Location:   time.UTC,
		DevLogin:   true,
	}
	key, err := secret.LoadKey("test secret", dir)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(database, key)
	assets := fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}}
	files, err := NewFiles(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxUploadMB = 5
	for _, f := range configure {
		f(cfg)
	}
	srv := New(cfg, st, auth.New(cfg, st, key), files, assets, "test")
	return &testEnv{h: srv.Handler(), st: st}
}

func call(h http.Handler, method, path, body string, c *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://wherehouse.test"+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeAs[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode %T: %v", v, err)
	}
	return v
}

func mustCall(t *testing.T, h http.Handler, c *http.Cookie, method, path, body string, want int) *httptest.ResponseRecorder {
	t.Helper()
	rec := call(h, method, path, body, c)
	if rec.Code != want {
		t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, want, rec.Body)
	}
	return rec
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 800, 400))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func upload(t *testing.T, h http.Handler, c *http.Cookie, path string, files map[string][]byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, data := range files {
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write(data)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "http://wherehouse.test"+path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// add makes a thing and returns it; kind is a built-in kind's key or "".
func add(t *testing.T, h http.Handler, c *http.Cookie, body string) store.Thing {
	t.Helper()
	return decodeAs[store.Thing](t, mustCall(t, h, c, "POST", "/api/things", body, 201))
}

// kindID returns the ID of a built-in kind in the current household.
func kindID(t *testing.T, h http.Handler, c *http.Cookie, key string) int64 {
	t.Helper()
	for _, k := range decodeAs[[]store.Kind](t, mustCall(t, h, c, "GET", "/api/kinds", "", 200)) {
		if k.Key == key {
			return k.ID
		}
	}
	t.Fatalf("no kind %q", key)
	return 0
}

func TestHealthAndSPA(t *testing.T) {
	h, _ := newTestServer(t)
	if rec := call(h, "GET", "/healthz", "", nil); rec.Code != 200 {
		t.Errorf("healthz = %d", rec.Code)
	}
	// Client routes get the app.
	if rec := call(h, "GET", "/places/12", "", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "doctype") {
		t.Errorf("client route = %d %q", rec.Code, rec.Body.String())
	}
}

func TestAPINeedsSession(t *testing.T) {
	h, _ := newTestServer(t)
	if rec := call(h, "GET", "/api/me", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/me without a session = %d, want 401", rec.Code)
	}
}

func TestSettings(t *testing.T) {
	h, c := newTestServer(t)

	rec := call(h, "GET", "/api/settings", "", c)
	if rec.Code != 200 {
		t.Fatalf("get settings = %d %s", rec.Code, rec.Body)
	}
	var got settingsResponse
	json.NewDecoder(rec.Body).Decode(&got)
	if got.Theme != "system" || got.Language != "" || !got.IsAdmin {
		t.Errorf("defaults = %+v", got)
	}

	rec = call(h, "PUT", "/api/settings", `{"theme":"dark","language":"cs"}`, c)
	if rec.Code != 200 {
		t.Fatalf("save settings = %d %s", rec.Code, rec.Body)
	}
	rec = call(h, "GET", "/api/settings", "", c)
	json.NewDecoder(rec.Body).Decode(&got)
	if got.Theme != "dark" || got.Language != "cs" {
		t.Errorf("after saving = %+v", got)
	}

	for _, body := range []string{`{"theme":"neon"}`, `{"theme":"dark","language":"xx"}`} {
		if rec := call(h, "PUT", "/api/settings", body, c); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", body, rec.Code)
		}
	}
}
