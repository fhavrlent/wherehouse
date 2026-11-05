package api

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/fhavrlent/wherehouse/internal/auth"
	"github.com/fhavrlent/wherehouse/internal/i18n"
	"github.com/fhavrlent/wherehouse/internal/store"
)

type householdKey struct{}

// inHousehold puts the household a request works in into its context: the
// API token's, or the one the person chose. Someone in none gets their own.
func (s *Server) inHousehold(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hh, ok := auth.TokenHousehold(r.Context())
		if !ok {
			lang := i18n.Match(r.Header.Get("Accept-Language"))
			if lang == "" {
				lang = i18n.Fallback
			}
			var err error
			if hh, err = s.store.CurrentHousehold(r.Context(), userID(r), i18n.T(lang, "server.defaultHousehold", nil)); err != nil {
				fail(w, err)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), householdKey{}, hh)))
	})
}

// household is the household a request works in.
func household(r *http.Request) int64 { return r.Context().Value(householdKey{}).(int64) }

// recentThings is how many recently changed things the home page shows.
const recentThings = 12

type homeResponse struct {
	// Places are the top-level places; Loose are top-level things that
	// aren't places.
	Places []store.Thing `json:"places"`
	Recent []store.Thing `json:"recent"`
	Loose  int           `json:"loose"`
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	hh := household(r)
	places, err := s.store.ListThings(r.Context(), hh, store.Filter{Top: true, Places: true})
	if err != nil {
		fail(w, err)
		return
	}
	all, err := s.store.ListThings(r.Context(), hh, store.Filter{Recent: true, Limit: recentThings * 3})
	if err != nil {
		fail(w, err)
		return
	}
	// Recently changed things, not the places they were put in.
	recent := []store.Thing{}
	for _, t := range all {
		if !t.IsPlace() && len(recent) < recentThings {
			recent = append(recent, t)
		}
	}
	loose, err := s.store.ListThings(r.Context(), hh, store.Filter{Unplaced: true})
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, homeResponse{Places: places, Recent: recent, Loose: len(loose)})
}

func (s *Server) listThings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.Filter{
		Top:       q.Get("top") == "1",
		Inside:    q.Get("inside") == "1",
		Unplaced:  q.Get("unplaced") == "1",
		Places:    q.Get("places") == "1",
		Archived:  q.Get("archived") == "1",
		Templates: q.Get("templates") == "1",
		Recent:    q.Get("recent") == "1",
	}
	for name, dst := range map[string]**int64{"in": &f.Parent, "tag": &f.TagID} {
		if v := q.Get(name); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeMsg(w, http.StatusBadRequest, "server.badId", nil)
				return
			}
			*dst = &id
		}
	}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	things, err := s.store.ListThings(r.Context(), household(r), f)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, things)
}

type thingResponse struct {
	*store.Thing
	// Contents are the things directly inside, places first.
	Contents []store.Thing `json:"contents"`
}

func (s *Server) getThing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	t, err := s.store.GetThing(r.Context(), household(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	contents, err := s.store.ListThings(r.Context(), household(r), store.Filter{Parent: &id})
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, thingResponse{Thing: t, Contents: contents})
}

func (s *Server) createThing(w http.ResponseWriter, r *http.Request) {
	var in store.ThingInput
	if !decode(w, r, &in) {
		return
	}
	t, err := s.store.CreateThing(r.Context(), household(r), userID(r), in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) updateThing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in store.ThingInput
	if !decode(w, r, &in) {
		return
	}
	t, err := s.store.UpdateThing(r.Context(), household(r), id, in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, t)
}

// move puts things inside another (null: in nothing).
func (s *Server) move(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Into   *int64  `json:"into"`
		Things []int64 `json:"things"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.store.Move(r.Context(), household(r), in.Things, in.Into); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) archiveThing(archived bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "id")
		if !ok {
			return
		}
		t, err := s.store.Archive(r.Context(), household(r), id, archived)
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, 200, t)
	}
}

func (s *Server) deleteThing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	files, err := s.store.DeleteThing(r.Context(), household(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	s.files.Remove(files...)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := s.store.Search(r.Context(), household(r), q.Get("q"), q.Get("archived") == "1")
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, res)
}

// copyThing copies a thing, makes a template of it, or a thing from a
// template; with photos, the photos are copied too.
func (s *Server) copyThing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		store.CopyInput
		Photos bool `json:"photos"`
	}
	if !decode(w, r, &in) {
		return
	}
	hh := household(r)
	t, err := s.store.CopyThing(r.Context(), hh, userID(r), id, in.CopyInput)
	if err != nil {
		fail(w, err)
		return
	}
	if in.Photos {
		src, err := s.store.GetThing(r.Context(), hh, id)
		if err != nil {
			fail(w, err)
			return
		}
		for _, p := range src.Photos {
			if err := s.copyPhoto(r, p, t.ID); err != nil {
				slog.Warn("copy photo", "photo", p.ID, "err", err)
			}
		}
		if t, err = s.store.GetThing(r.Context(), hh, t.ID); err != nil {
			fail(w, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) copyPhoto(r *http.Request, p store.Photo, to int64) error {
	f, err := s.files.Open(p.File)
	if err != nil {
		return err
	}
	defer f.Close()
	name, size, err := s.files.Save(f, filepath.Ext(p.File))
	if err != nil {
		return err
	}
	if err := s.store.AddPhoto(r.Context(), household(r), userID(r), &store.Photo{ThingID: to, File: name, MimeType: p.MimeType, Size: size}); err != nil {
		s.files.Remove(name)
		return err
	}
	return nil
}
