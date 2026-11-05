package api

import (
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
	var got map[string]any
	json.Unmarshal(mustCall(t, h, c, "GET", "/api/settings", "", 200).Body.Bytes(), &got)
	if got["paperlessBacklinks"] != true {
		t.Errorf("settings = %v", got)
	}
}
