package api

import (
	"net/http"
	"time"

	"github.com/fhavrlent/wherehouse/internal/store"
)

// setAppLink links a thing to what it is in another app; see store.SetAppLink.
func (s *Server) setAppLink(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in store.AppLinkInput
	if !decode(w, r, &in) {
		return
	}
	if err := s.store.SetAppLink(r.Context(), household(r), id, r.PathValue("source"), in); err != nil {
		fail(w, err)
		return
	}
	s.getThing(w, r)
}

func (s *Server) removeAppLink(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.RemoveAppLink(r.Context(), household(r), id, r.PathValue("source")); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// thingByApp finds the thing linked to something of an app.
func (s *Server) thingByApp(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.ThingByApp(r.Context(), household(r), r.PathValue("source"), r.PathValue("external"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, t)
}

// thingByHomeBox finds the thing a HomeBox item or location was imported
// as, so links to HomeBox can be moved here.
func (s *Server) thingByHomeBox(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.FromHomeBox(r.Context(), household(r), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, t)
}

// changes lists what changed since ?since (RFC 3339; default the last day),
// for apps keeping in step. Ask again from the answer's "until".
func (s *Server) changes(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-24 * time.Hour)
	if v := r.URL.Query().Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeMsg(w, http.StatusBadRequest, "server.badRequest", nil)
			return
		}
		since = t
	}
	c, err := s.store.ChangesSince(r.Context(), household(r), since)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, c)
}
