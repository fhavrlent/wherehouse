package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fhavrlent/wherehouse/internal/store"
)

type householdsResponse struct {
	Current    int64             `json:"current"`
	Households []store.Household `json:"households"`
}

func (s *Server) listHouseholds(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListHouseholds(r.Context(), userID(r))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, householdsResponse{Current: household(r), Households: list})
}

func (s *Server) createHousehold(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name, Currency string }
	if !decode(w, r, &in) {
		return
	}
	if _, err := s.store.CreateHousehold(r.Context(), userID(r), in.Name, in.Currency); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) switchHousehold(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.SwitchHousehold(r.Context(), userID(r), id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type householdResponse struct {
	*store.Household
	Members []store.Member `json:"members"`
}

// currentHousehold describes the household a request works in, with who
// is in it.
func (s *Server) currentHousehold(w http.ResponseWriter, r *http.Request) {
	h, err := s.store.GetHousehold(r.Context(), household(r))
	if err != nil {
		fail(w, err)
		return
	}
	if h.Role, err = s.store.Role(r.Context(), h.ID, userID(r)); err != nil {
		fail(w, err)
		return
	}
	members, err := s.store.Members(r.Context(), h.ID)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, householdResponse{Household: h, Members: members})
}

// owner refuses requests from anyone but an owner of the household, and
// from API tokens.
func (s *Server) owner(w http.ResponseWriter, r *http.Request) bool {
	role, err := s.store.Role(r.Context(), household(r), userID(r))
	if err != nil {
		fail(w, err)
		return false
	}
	if role != store.RoleOwner || isToken(r) {
		writeMsg(w, http.StatusForbidden, "server.ownerOnly", nil)
		return false
	}
	return true
}

func (s *Server) updateHousehold(w http.ResponseWriter, r *http.Request) {
	if !s.owner(w, r) {
		return
	}
	var in struct{ Name, Currency string }
	if !decode(w, r, &in) {
		return
	}
	if err := s.store.UpdateHousehold(r.Context(), household(r), in.Name, in.Currency); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createInvite(w http.ResponseWriter, r *http.Request) {
	if !s.owner(w, r) {
		return
	}
	raw, expires, err := s.store.CreateInvite(r.Context(), household(r), userID(r))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"url":     strings.TrimSuffix(s.cfg.AppURL.String(), "/") + "/join/" + raw,
		"expires": expires.Format(time.RFC3339),
	})
}

func (s *Server) getInvite(w http.ResponseWriter, r *http.Request) {
	h, err := s.store.Invite(r.Context(), r.PathValue("token"))
	if err != nil {
		writeMsg(w, http.StatusNotFound, "server.inviteGone", nil)
		return
	}
	_, member := s.store.Role(r.Context(), h.ID, userID(r))
	writeJSON(w, 200, map[string]any{"name": h.Name, "members": h.Members, "member": member == nil})
}

func (s *Server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	if isToken(r) {
		writeMsg(w, http.StatusForbidden, "server.ownerOnly", nil)
		return
	}
	if _, err := s.store.AcceptInvite(r.Context(), r.PathValue("token"), userID(r)); err != nil {
		if err == store.ErrNotFound {
			writeMsg(w, http.StatusNotFound, "server.inviteGone", nil)
			return
		}
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func memberID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("user"), 10, 64)
	if err != nil {
		writeMsg(w, http.StatusBadRequest, "server.badId", nil)
		return 0, false
	}
	return id, true
}

// removeMember takes someone out of the household; anyone may leave.
func (s *Server) removeMember(w http.ResponseWriter, r *http.Request) {
	uid, ok := memberID(w, r)
	if !ok {
		return
	}
	if uid != userID(r) && !s.owner(w, r) {
		return
	}
	files, err := s.store.RemoveMember(r.Context(), household(r), uid)
	if err != nil {
		fail(w, err)
		return
	}
	s.files.Remove(files...)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setRole(w http.ResponseWriter, r *http.Request) {
	uid, ok := memberID(w, r)
	if !ok || !s.owner(w, r) {
		return
	}
	var in struct{ Role string }
	if !decode(w, r, &in) {
		return
	}
	if err := s.store.SetRole(r.Context(), household(r), uid, in.Role); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
