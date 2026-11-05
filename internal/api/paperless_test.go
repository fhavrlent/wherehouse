package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fhavrlent/wherehouse/internal/config"
	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestPaperlessLinkBack(t *testing.T) {
	var mu sync.Mutex
	var patched []string
	pl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "PATCH" && r.URL.Path == "/api/documents/7/":
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			patched = append(patched, string(b))
			mu.Unlock()
			w.Write([]byte(`{}`))
		case r.URL.Path == "/api/documents/7/" && r.URL.Query().Get("fields") == "custom_fields":
			w.Write([]byte(`{"custom_fields":[]}`))
		case r.URL.Path == "/api/documents/7/preview/":
			w.WriteHeader(http.StatusInternalServerError)
		case r.URL.Path == "/api/documents/7/":
			w.Write([]byte(`{"id":7,"title":"Receipt","original_file_name":"r.pdf","mime_type":"application/pdf","created":"2026-01-01"}`))
		case r.URL.Path == "/api/custom_fields/" && r.Method == "GET":
			w.Write([]byte(`{"results":[{"id":3,"name":"Wherehouse","data_type":"url"}]}`))
		case r.URL.Path == "/api/documents/":
			w.Write([]byte(`{"count":1,"results":[{"id":7}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer pl.Close()

	e := newTestEnv(t, func(c *config.Config) {
		c.Paperless = config.Paperless{URL: pl.URL, PublicURL: pl.URL, Field: "Wherehouse"}
	})
	c := e.signIn(t, "a")
	h := e.h
	mustCall(t, h, c, "PUT", "/api/settings", `{"theme":"system","notifyDays":"7","paperlessToken":"tok","paperlessBacklinks":true}`, 200)
	tv := add(t, h, c, `{"name":"TV"}`)
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/attachments/paperless", tv.ID), `{"documentIds":[7],"kind":"receipt"}`, 201)

	// The link back is written in the background.
	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		n := len(patched)
		mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(patched) != 1 || !strings.Contains(patched[0], `http://wherehouse.test/d/7`) || !strings.Contains(patched[0], `"field":3`) {
		t.Fatalf("patched = %q", patched)
	}

	things := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/paperless/documents/7/things", "", 200))
	if len(things) != 1 || things[0].ID != tv.ID {
		t.Errorf("things with the document = %+v", things)
	}
	// A document Paperless won't hand over here opens in Paperless itself.
	tv = decodeAs[store.Thing](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", tv.ID), "", 200))
	rec := mustCall(t, h, c, "GET", fmt.Sprintf("/api/attachments/%d/file", tv.Attachments[0].ID), "", http.StatusFound)
	if loc := rec.Header().Get("Location"); loc != pl.URL+"/documents/7/details" {
		t.Errorf("opened at %q", loc)
	}

	var got map[string]any
	json.Unmarshal(mustCall(t, h, c, "GET", "/api/settings", "", 200).Body.Bytes(), &got)
	if got["paperlessBacklinks"] != true {
		t.Errorf("settings = %v", got)
	}
}

func TestTokenNeverUsesServerPaperless(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	pl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/documents/" {
			w.Write([]byte(`{"count":0,"results":[]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer pl.Close()
	usedServerToken := func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, a := range seen {
			if a == "Token server-token" {
				return true
			}
		}
		return false
	}

	e := newTestEnv(t, func(c *config.Config) {
		c.Paperless = config.Paperless{URL: pl.URL, PublicURL: pl.URL, Token: "server-token"}
	})
	c := e.signIn(t, "a")
	h := e.h
	me := decodeAs[store.User](t, mustCall(t, h, c, "GET", "/api/me", "", 200))
	if err := e.st.SetAdmin(context.Background(), me.ID, true); err != nil {
		t.Fatal(err)
	}
	token := decodeAs[createdToken](t, mustCall(t, h, c, "POST", "/api/tokens", `{"name":"App"}`, 201)).Secret

	// The admin's token can't search with the server's Paperless token…
	if rec := callWithToken(h, "GET", "/api/paperless/search", "", token); rec.Code != http.StatusBadRequest {
		t.Errorf("search with a token = %d %s", rec.Code, rec.Body)
	}
	// …nor open a document the admin linked: it opens in Paperless instead.
	tv := add(t, h, c, `{"name":"TV"}`)
	hh := decodeAs[householdsResponse](t, mustCall(t, h, c, "GET", "/api/households", "", 200)).Current
	doc := int64(7)
	a := &store.Attachment{ThingID: tv.ID, Kind: "receipt", Storage: store.StoragePaperless, Filename: "r.pdf",
		MimeType: "application/pdf", PaperlessDocID: &doc, AddedBy: &me.ID}
	if err := e.st.AddAttachment(context.Background(), hh, a); err != nil {
		t.Fatal(err)
	}
	if rec := callWithToken(h, "GET", fmt.Sprintf("/api/attachments/%d/file", a.ID), "", token); rec.Code != http.StatusFound {
		t.Errorf("document with a token = %d", rec.Code)
	}
	if usedServerToken() {
		t.Fatal("a token request used the server's Paperless token")
	}

	// The admin signed in still can.
	mustCall(t, h, c, "GET", "/api/paperless/search", "", 200)
	if !usedServerToken() {
		t.Error("the admin's session didn't use the server's token")
	}
}
