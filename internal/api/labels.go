package api

import (
	"net/http"

	"github.com/fhavrlent/wherehouse/internal/store"
)

type labelResponse struct {
	Code  string       `json:"code"`
	Thing *store.Thing `json:"thing,omitempty"`
	// Household is set when the label is of another of the reader's
	// households, which they can switch to.
	Household *store.Household `json:"household,omitempty"`
}

// getLabel says what a scanned label is on, if anything.
func (s *Server) getLabel(w http.ResponseWriter, r *http.Request) {
	l, err := s.store.GetLabel(r.Context(), household(r), r.PathValue("code"))
	if err == store.ErrNotFound {
		// A label of another of the reader's households.
		if hh, herr := s.store.LabelHousehold(r.Context(), r.PathValue("code")); herr == nil {
			if _, rerr := s.store.Role(r.Context(), hh, userID(r)); rerr == nil {
				h, gerr := s.store.GetHousehold(r.Context(), hh)
				if gerr == nil {
					code, _ := store.NormalizeCode(r.PathValue("code"))
					writeJSON(w, 200, labelResponse{Code: code, Household: h})
					return
				}
			}
		}
	}
	if err != nil {
		fail(w, err)
		return
	}
	out := labelResponse{Code: l.Code}
	if l.ThingID != nil {
		if out.Thing, err = s.store.GetThing(r.Context(), household(r), *l.ThingID); err != nil {
			fail(w, err)
			return
		}
	}
	writeJSON(w, 200, out)
}

func (s *Server) blankLabels(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Count int `json:"count"`
	}
	if !decode(w, r, &in) {
		return
	}
	codes, err := s.store.BlankLabels(r.Context(), household(r), in.Count)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string][]string{"codes": codes})
}

// attachLabel puts a label on a thing; without a code, it gets the one it
// has or a new one.
func (s *Server) attachLabel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code    string `json:"code"`
		ThingID int64  `json:"thingId"`
	}
	if !decode(w, r, &in) {
		return
	}
	code, err := s.store.AttachLabel(r.Context(), household(r), in.ThingID, in.Code)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"code": code})
}

func (s *Server) detachLabel(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DetachLabel(r.Context(), household(r), r.PathValue("code")); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
