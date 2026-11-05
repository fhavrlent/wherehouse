package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/fhavrlent/wherehouse/internal/auth"
	"github.com/fhavrlent/wherehouse/internal/store"
)

// An app connected through /connect can ask to be pinged when something in
// its household changes, so it needn't wait for its next look at the change
// feed. The ping carries no data, only that there is something to read; it
// goes only to the host the person approved when connecting, is signed with
// the hook's secret (X-Wherehouse-Signature: sha256=<HMAC of the body>), and
// isn't retried: a missed ping is caught by the app's next regular read.

// hookDelay gathers the changes of a moment into one ping.
const hookDelay = 3 * time.Second

type hooks struct {
	store  *store.Store
	client *http.Client
	mu     sync.Mutex
	// pending are the tokens with a ping on its way, so one burst of
	// changes sends one.
	pending map[int64]bool
}

func newHooks(st *store.Store) *hooks {
	return &hooks{store: st, pending: map[int64]bool{},
		client: &http.Client{Timeout: 10 * time.Second,
			// The approved host is the only one pinged; a redirect elsewhere
			// isn't followed.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// changed pings the household's apps, except the one that made the change.
func (h *hooks) changed(hh, except int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	list, err := h.store.Hooks(ctx, hh)
	if err != nil {
		slog.Warn("hooks", "err", err)
		return
	}
	for _, hook := range list {
		if hook.TokenID == except {
			continue
		}
		h.mu.Lock()
		if h.pending[hook.TokenID] {
			h.mu.Unlock()
			continue
		}
		h.pending[hook.TokenID] = true
		h.mu.Unlock()
		time.AfterFunc(hookDelay, func() {
			h.mu.Lock()
			delete(h.pending, hook.TokenID)
			h.mu.Unlock()
			h.send(hook, hh)
		})
	}
}

func (h *hooks) send(hook store.Hook, hh int64) {
	body, _ := json.Marshal(map[string]any{"event": "changed", "household": hh, "at": time.Now().UTC().Format(time.RFC3339)})
	req, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Wherehouse-Signature", Sign(hook.Secret, body))
	res, err := h.client.Do(req)
	if err != nil {
		slog.Debug("hook not delivered", "token", hook.TokenID, "err", err)
		return
	}
	res.Body.Close()
}

// Sign is a ping's signature: "sha256=" and the hex HMAC of the body.
func Sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

// pingOnChange pings the household's apps after a request that changed
// something.
func (s *Server) pingOnChange(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if r.Method == http.MethodGet || r.Method == http.MethodHead || sw.status >= 400 {
			return
		}
		var except int64
		if t, ok := auth.Token(r.Context()); ok {
			except = t.ID
		}
		go s.hooks.changed(household(r), except)
	})
}

// setHook registers where to ping the app whose token this is. Only tokens
// from connecting can, and only at the host the person approved.
func (s *Server) setHook(w http.ResponseWriter, r *http.Request) {
	t, ok := auth.Token(r.Context())
	if !ok {
		writeMsg(w, http.StatusForbidden, "server.hookTokenOnly", nil)
		return
	}
	var in struct {
		URL string `json:"url"`
	}
	if !decode(w, r, &in) {
		return
	}
	host, err := s.store.TokenAppHost(r.Context(), t.ID)
	if err != nil {
		fail(w, err)
		return
	}
	u, err := url.Parse(strings.TrimSpace(in.URL))
	switch {
	case host == "":
		writeMsg(w, http.StatusForbidden, "server.hookNotConnected", nil)
		return
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, host) || u.User != nil:
		writeMsg(w, http.StatusBadRequest, "server.hookHost", map[string]any{"host": host})
		return
	}
	secret, err := s.store.SetHook(r.Context(), t.ID, u.String())
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"secret": secret})
}

func (s *Server) removeHook(w http.ResponseWriter, r *http.Request) {
	t, ok := auth.Token(r.Context())
	if !ok {
		writeMsg(w, http.StatusForbidden, "server.hookTokenOnly", nil)
		return
	}
	if _, err := s.store.SetHook(r.Context(), t.ID, ""); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
