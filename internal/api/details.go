package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/fhavrlent/wherehouse/internal/store"
)

// ---- tags ----

func (s *Server) listTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.store.ListTags(r.Context(), household(r))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, tags)
}

type tagResponse struct {
	*store.Tag
	Things []store.Thing `json:"things"`
}

func (s *Server) getTag(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	t, err := s.store.GetTag(r.Context(), household(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	things, err := s.store.ListThings(r.Context(), household(r), store.Filter{TagID: &id})
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, tagResponse{Tag: t, Things: things})
}

func (s *Server) updateTag(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct{ Name, Color string }
	if !decode(w, r, &in) {
		return
	}
	if err := s.store.UpdateTag(r.Context(), household(r), id, in.Name, in.Color); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteTag(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteTag(r.Context(), household(r), id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- custom fields ----

func (s *Server) listFields(w http.ResponseWriter, r *http.Request) {
	fields, err := s.store.ListFields(r.Context(), household(r))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, fields)
}

func (s *Server) createField(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name, Type string }
	if !decode(w, r, &in) {
		return
	}
	f, err := s.store.CreateField(r.Context(), household(r), in.Name, in.Type)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) updateField(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Name     string `json:"name"`
		Position int    `json:"position"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.store.UpdateField(r.Context(), household(r), id, in.Name, in.Position); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteField(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteField(r.Context(), household(r), id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- maintenance ----

func (s *Server) addMaintenance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in store.MaintenanceInput
	if !decode(w, r, &in) {
		return
	}
	m, err := s.store.AddMaintenance(r.Context(), household(r), userID(r), id, in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateMaintenance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in store.MaintenanceInput
	if !decode(w, r, &in) {
		return
	}
	m, err := s.store.UpdateMaintenance(r.Context(), household(r), id, in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, m)
}

func (s *Server) deleteMaintenance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteMaintenance(r.Context(), household(r), id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// dueMaintenance lists what is due within ?days (default 30) days, and
// anything overdue.
func (s *Server) dueMaintenance(w http.ResponseWriter, r *http.Request) {
	days := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v >= 0 && v <= 3650 {
		days = v
	}
	y, m, d := time.Now().In(s.cfg.Location).Date()
	until := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, days).Format(store.DateLayout)
	due, err := s.store.DueMaintenance(r.Context(), household(r), until)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, due)
}
