package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/fhavrlent/wherehouse/internal/store"
)

// Another app connects like this:
//
//  1. It sends the person to /connect?app=<name>&redirect_uri=<its URL>&state=<anything>.
//  2. Signed in here, they pick a household and allow it; that page posts to
//     /api/connect, and the browser goes back to redirect_uri with ?code and ?state.
//  3. The app, server to server, posts {code, redirect_uri} to /connect/token
//     and gets an API token for that person and household.
//
// A code works once, for connectCodeTTL, and only with the same redirect_uri.

const connectCodeTTL = 5 * time.Minute

type connectGrant struct {
	user, household int64
	app, redirect   string
	expires         time.Time
}

// host is the address the person approved, which the app's pings may go to.
func (g connectGrant) host() string {
	u, _ := url.Parse(g.redirect)
	return u.Host
}

type connectCodes struct {
	mu    sync.Mutex
	codes map[string]connectGrant
}

func (c *connectCodes) add(g connectGrant) string {
	b := make([]byte, 24)
	rand.Read(b)
	code := hex.EncodeToString(b)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.codes == nil {
		c.codes = map[string]connectGrant{}
	}
	for k, v := range c.codes {
		if time.Now().After(v.expires) {
			delete(c.codes, k)
		}
	}
	c.codes[code] = g
	return code
}

// take returns and forgets the grant of a code given with the same redirect.
func (c *connectCodes) take(code, redirect string) (connectGrant, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g, ok := c.codes[code]
	if !ok {
		return connectGrant{}, false
	}
	delete(c.codes, code)
	if time.Now().After(g.expires) || subtle.ConstantTimeCompare([]byte(g.redirect), []byte(redirect)) != 1 {
		return connectGrant{}, false
	}
	return g, true
}

// validRedirect accepts web addresses without fragments.
func validRedirect(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.Fragment == "" && len(raw) <= 2000
}

// allowConnect is where the person allows an app; it answers with where to
// send the browser.
func (s *Server) allowConnect(w http.ResponseWriter, r *http.Request) {
	if isToken(r) {
		writeMsg(w, http.StatusForbidden, "server.ownerOnly", nil)
		return
	}
	var in struct {
		App         string `json:"app"`
		RedirectURI string `json:"redirectUri"`
		State       string `json:"state"`
		Household   int64  `json:"household"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.App = strings.TrimSpace(in.App)
	if in.App == "" || utf8.RuneCountInString(in.App) > store.MaxTagName || !validRedirect(in.RedirectURI) || len(in.State) > 500 {
		writeMsg(w, http.StatusBadRequest, "server.connectBad", nil)
		return
	}
	if _, err := s.store.Role(r.Context(), in.Household, userID(r)); err != nil {
		fail(w, err)
		return
	}
	code := s.connect.add(connectGrant{user: userID(r), household: in.Household, app: in.App, redirect: in.RedirectURI,
		expires: time.Now().Add(connectCodeTTL)})
	u, _ := url.Parse(in.RedirectURI)
	q := u.Query()
	q.Set("code", code)
	q.Set("state", in.State)
	u.RawQuery = q.Encode()
	writeJSON(w, 200, map[string]string{"redirect": u.String()})
}

// connectToken swaps a code for an API token. It is called by the other
// app's server, not a browser, so it takes no session.
func (s *Server) connectToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
	}
	if !decode(w, r, &in) {
		return
	}
	g, ok := s.connect.take(in.Code, in.RedirectURI)
	if !ok {
		writeMsg(w, http.StatusBadRequest, "server.connectCode", nil)
		return
	}
	raw, tok, err := s.store.CreateToken(r.Context(), g.user, g.household, g.app)
	if err != nil {
		fail(w, err)
		return
	}
	if err := s.store.SetTokenAppHost(r.Context(), tok.ID, g.host()); err != nil {
		fail(w, err)
		return
	}
	h, err := s.store.GetHousehold(r.Context(), g.household)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"token": raw, "household": map[string]any{"id": h.ID, "name": h.Name}})
}
