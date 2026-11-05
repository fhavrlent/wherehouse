package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fhavrlent/wherehouse/internal/homebox"
	"github.com/fhavrlent/wherehouse/internal/importer"
)

// importHomeBox starts copying a HomeBox inventory, in the background.
// Only admins may, since the server calls the address they give.
func (s *Server) importHomeBox(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeMsg(w, http.StatusForbidden, "server.adminOnly", nil)
		return
	}
	var in struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.URL = strings.TrimSuffix(strings.TrimSpace(in.URL), "/")
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		writeMsg(w, http.StatusBadRequest, "server.homeboxUrl", nil)
		return
	}
	hb := homebox.New(in.URL, strings.TrimSpace(in.Token))
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if _, err := hb.Self(ctx); err != nil {
		var he *homebox.Error
		if errors.As(err, &he) && he.Unauthorized() {
			writeMsg(w, http.StatusBadRequest, "server.homeboxToken", nil)
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	s.importMu.Lock()
	defer s.importMu.Unlock()
	if s.importer != nil && s.importer.Progress().Running {
		writeMsg(w, http.StatusConflict, "server.importRunning", nil)
		return
	}
	im := importer.New(s.store, hb, s.files, household(r), userID(r))
	s.importer = im
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		defer cancel()
		if err := im.Run(ctx); err != nil {
			slog.Error("homebox import failed", "err", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, im.Progress())
}

// importStatus reports how the last import is going, or null when there
// was none since the server started.
func (s *Server) importStatus(w http.ResponseWriter, r *http.Request) {
	s.importMu.Lock()
	im := s.importer
	s.importMu.Unlock()
	if im == nil {
		writeJSON(w, 200, nil)
		return
	}
	writeJSON(w, 200, im.Progress())
}
