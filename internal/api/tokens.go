package api

import (
	"net/http"

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
