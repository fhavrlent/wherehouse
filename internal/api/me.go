package api

import (
	"net/http"

	"github.com/fhavrlent/wherehouse/internal/auth"
)

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, auth.UserFrom(r.Context()))
}

func (s *Server) mePicture(w http.ResponseWriter, r *http.Request) {
	data, contentType, err := s.store.UserPicture(r.Context(), userID(r))
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	// The URL carries the picture's hash, so it never changes under the same URL.
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Write(data)
}
