package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/fhavrlent/wherehouse/internal/auth"
	"github.com/fhavrlent/wherehouse/internal/store"
)

// Another app connects like this:
//
//  1. It makes a random code_verifier and sends the person to
//     /connect?app=<name>&redirect_uri=<its URL>&state=<anything>
//     &code_challenge=<base64url SHA-256 of the verifier>&code_challenge_method=S256.
//  2. Signed in here, they pick a household and allow it; that page posts to
//     /api/connect, and the browser goes back to redirect_uri with ?code and ?state.
//  3. The app, server to server, posts {code, redirect_uri, code_verifier} to
//     /connect/token and gets an API token for that person and household.
//
// With &multiple=1 the person ticks one or more households, and step 3
// answers with a token for each: {tokens: [{household, token}]}. Connecting
// a household the app at that host is already connected to, either way,
// replaces its old token; the page lets the person tick such a household
// only when the app names it in &household=, to reconnect it. An app token lists the person's households, and which of them the
// app is connected to, at GET /api/connect/households.
//
// A code works once, for connectCodeTTL, only with the same redirect_uri,
// and only with the verifier (PKCE, RFC 7636): a code that leaks, e.g. in a
// log of the app's callback, is no use to anyone else.

const connectCodeTTL = 5 * time.Minute

type connectGrant struct {
	user, household int64
	// households are the ones ticked with multiple, which answers with a
	// token for each.
	households               []int64
	multiple                 bool
	app, redirect, challenge string
	expires                  time.Time
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

// take returns and forgets the grant of a code given with the same redirect
// and the verifier of its challenge. A code is used up by any try.
func (c *connectCodes) take(code, redirect, verifier string) (connectGrant, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g, ok := c.codes[code]
	if !ok {
		return connectGrant{}, false
	}
	delete(c.codes, code)
	if time.Now().After(g.expires) || subtle.ConstantTimeCompare([]byte(g.redirect), []byte(redirect)) != 1 ||
		!verifierRe.MatchString(verifier) || subtle.ConstantTimeCompare([]byte(challengeOf(verifier)), []byte(g.challenge)) != 1 {
		return connectGrant{}, false
	}
	return g, true
}

// A PKCE verifier is 43 to 128 unreserved characters; its S256 challenge is
// the unpadded base64url SHA-256 of it, 43 characters.
var (
	verifierRe  = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	challengeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

func challengeOf(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
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
		// CodeChallenge is the app's S256 PKCE challenge.
		CodeChallenge string `json:"codeChallenge"`
		Household     int64  `json:"household"`
		// Multiple asks for a token for each of Households.
		Multiple   bool    `json:"multiple"`
		Households []int64 `json:"households"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.App = strings.TrimSpace(in.App)
	if in.App == "" || utf8.RuneCountInString(in.App) > store.MaxTagName || !validRedirect(in.RedirectURI) || len(in.State) > 500 ||
		!challengeRe.MatchString(in.CodeChallenge) {
		writeMsg(w, http.StatusBadRequest, "server.connectBad", nil)
		return
	}
	g := connectGrant{user: userID(r), household: in.Household, app: in.App, redirect: in.RedirectURI,
		challenge: in.CodeChallenge, expires: time.Now().Add(connectCodeTTL)}
	if in.Multiple {
		g.multiple, g.household = true, 0
		for _, hh := range in.Households {
			if !slices.Contains(g.households, hh) {
				g.households = append(g.households, hh)
			}
		}
		if len(g.households) == 0 || len(g.households) > store.MaxHouseholds {
			writeMsg(w, http.StatusBadRequest, "server.connectBad", nil)
			return
		}
	} else {
		g.households = []int64{in.Household}
	}
	for _, hh := range g.households {
		if _, err := s.store.Role(r.Context(), hh, userID(r)); err != nil {
			fail(w, err)
			return
		}
	}
	code := s.connect.add(g)
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
		Code         string `json:"code"`
		RedirectURI  string `json:"redirect_uri"`
		CodeVerifier string `json:"code_verifier"`
	}
	if !decode(w, r, &in) {
		return
	}
	g, ok := s.connect.take(in.Code, in.RedirectURI, in.CodeVerifier)
	if !ok {
		writeMsg(w, http.StatusBadRequest, "server.connectCode", nil)
		return
	}
	if !g.multiple {
		made, err := s.store.CreateAppTokens(r.Context(), g.user, g.app, g.host(), []int64{g.household}, true)
		if err != nil {
			fail(w, err)
			return
		}
		if len(made) == 0 {
			fail(w, store.ErrNotFound)
			return
		}
		writeJSON(w, 200, connectedToken{Token: made[0].Token, Household: householdBrief{ID: made[0].Household, Name: made[0].HouseholdName}})
		return
	}
	// All households' tokens or none, so a failure leaves no token the app
	// never got.
	made, err := s.store.CreateAppTokens(r.Context(), g.user, g.app, g.host(), g.households, true)
	if err != nil {
		fail(w, err)
		return
	}
	tokens := make([]connectedToken, 0, len(made))
	for _, m := range made {
		tokens = append(tokens, connectedToken{Token: m.Token, Household: householdBrief{ID: m.Household, Name: m.HouseholdName}})
	}
	writeJSON(w, 200, map[string]any{"tokens": tokens})
}

type connectedToken struct {
	Token     string         `json:"token"`
	Household householdBrief `json:"household"`
}

type householdBrief struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type connectHousehold struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Members   int    `json:"members"`
	Connected bool   `json:"connected"`
}

// connectHouseholds lists the person's households and whether the app is
// connected to each: with an app's token, that app at its host; signed in,
// the app and redirect_uri of the connect page asking.
func (s *Server) connectHouseholds(w http.ResponseWriter, r *http.Request) {
	var app, host string
	if t, ok := auth.Token(r.Context()); ok {
		var err error
		if app, host, err = s.store.TokenApp(r.Context(), t.ID); err != nil {
			fail(w, err)
			return
		}
		// A token made by hand sees only its own household.
		if host == "" {
			writeMsg(w, http.StatusForbidden, "server.connectedAppOnly", nil)
			return
		}
	} else {
		q := r.URL.Query()
		if !validRedirect(q.Get("redirect_uri")) {
			writeMsg(w, http.StatusBadRequest, "server.connectBad", nil)
			return
		}
		app = q.Get("app")
		host = connectGrant{redirect: q.Get("redirect_uri")}.host()
	}
	list, err := s.store.ListHouseholds(r.Context(), userID(r))
	if err != nil {
		fail(w, err)
		return
	}
	connected, err := s.store.AppHouseholds(r.Context(), userID(r), app, host)
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]connectHousehold, 0, len(list))
	for _, h := range list {
		out = append(out, connectHousehold{ID: h.ID, Name: h.Name, Members: h.Members, Connected: connected[h.ID]})
	}
	writeJSON(w, 200, out)
}
