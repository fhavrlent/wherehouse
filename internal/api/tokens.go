package api

import (
	"net/http"

	"github.com/fhavrlent/wherehouse/internal/auth"
	"github.com/fhavrlent/wherehouse/internal/store"
)

func (s *Server) listTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.store.ListTokens(r.Context(), userID(r))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, tokens)
}

type createdToken struct {
	*store.Token
	Secret string `json:"token"`
}

// createToken makes an API token; the response is the only time it is shown.
func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	raw, t, err := s.store.CreateToken(r.Context(), userID(r), household(r), in.Name)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, createdToken{Token: t, Secret: raw})
}

func (s *Server) deleteToken(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteToken(r.Context(), userID(r), id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// revokeOwnToken ends the API token the request was made with, e.g. when
// an app disconnects.
func (s *Server) revokeOwnToken(w http.ResponseWriter, r *http.Request) {
	t, ok := auth.Token(r.Context())
	if !ok {
		writeMsg(w, http.StatusForbidden, "server.tokenOnly", nil)
		return
	}
	if err := s.store.DeleteToken(r.Context(), userID(r), t.ID); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// appRoutes are what a token from connecting an app may do: what an app
// keeping warranties, receipts or the like in step needs, and nothing that
// takes the whole household out (exports) or changes how it is set up. A
// token made by hand, for the person's own scripts, may do everything.
var appRoutes = map[string]bool{
	"GET /api/me":        true,
	"GET /api/household": true,
	"GET /api/search":    true,
	"GET /api/changes":   true,

	"GET /api/things":                    true,
	"POST /api/things":                   true,
	"GET /api/things/{id}":               true,
	"PATCH /api/things/{id}":             true,
	"DELETE /api/things/{id}":            true,
	"POST /api/things/{id}/photos":       true,
	"POST /api/things/{id}/links":        true,
	"POST /api/things/{id}/maintenance":  true,
	"PUT /api/things/{id}/apps/{source}": true,

	"DELETE /api/things/{id}/apps/{source}":       true,
	"POST /api/things/{id}/attachments/paperless": true,
	"GET /api/apps/{source}/{external}":           true,
	"GET /api/homebox/{id}":                       true,

	"GET /api/photos/{id}":           true,
	"DELETE /api/photos/{id}":        true,
	"GET /api/attachments/{id}/file": true,
	"PATCH /api/attachments/{id}":    true,
	"DELETE /api/attachments/{id}":   true,
	"GET /api/paperless":             true,
	"PUT /api/maintenance/{id}":      true,
	"DELETE /api/maintenance/{id}":   true,
	"PUT /api/hook":                  true,
	"DELETE /api/hook":               true,
	"DELETE /api/token":              true,
}

// appScope refuses an app's token what isn't in appRoutes.
func appScope(api *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if t, ok := auth.Token(r.Context()); ok && t.App {
			if _, pattern := api.Handler(r); pattern != "" && !appRoutes[pattern] {
				writeMsg(w, http.StatusForbidden, "server.appNotAllowed", nil)
				return
			}
		}
		api.ServeHTTP(w, r)
	})
}

func (s *Server) addLink(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if !decode(w, r, &in) {
		return
	}
	l, err := s.store.AddLink(r.Context(), household(r), id, in.URL, in.Title)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteLink(r.Context(), household(r), id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
