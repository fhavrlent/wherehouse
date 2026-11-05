package api

import (
	"net/http"
	"strconv"

	"github.com/fhavrlent/wherehouse/internal/store"
)

// report sums up the household's things, or those inside ?in=<id>.
func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	var in *int64
	if v := r.URL.Query().Get("in"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeMsg(w, http.StatusBadRequest, "server.badId", nil)
			return
		}
		if _, err := s.store.GetThing(r.Context(), household(r), id); err != nil {
			fail(w, err)
			return
		}
		in = &id
	}
	rep, err := s.store.Report(r.Context(), household(r), in, s.cfg.Location)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rep)
}

// insured lists the insured things that haven't been sold, for an
// insurance list.
func (s *Server) insured(w http.ResponseWriter, r *http.Request) {
	all, err := s.store.ListThings(r.Context(), household(r), store.Filter{})
	if err != nil {
		fail(w, err)
		return
	}
	out := []store.Thing{}
	for _, t := range all {
		if t.Insured && t.SoldDate == nil {
			out = append(out, t)
		}
	}
	writeJSON(w, 200, out)
}
