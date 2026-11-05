package api

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/fhavrlent/wherehouse/internal/i18n"
	"github.com/fhavrlent/wherehouse/internal/store"
)

func (s *Server) listKinds(w http.ResponseWriter, r *http.Request) {
	kinds, err := s.store.ListKinds(r.Context(), household(r))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, kinds)
}

func (s *Server) createKind(w http.ResponseWriter, r *http.Request) {
	var in store.KindInput
	if !decode(w, r, &in) {
		return
	}
	k, err := s.store.CreateKind(r.Context(), household(r), in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, k)
}

func (s *Server) updateKind(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in store.KindInput
	if !decode(w, r, &in) {
		return
	}
	k, err := s.store.UpdateKind(r.Context(), household(r), id, in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, k)
}

func (s *Server) deleteKind(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteKind(r.Context(), household(r), id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// maxIconBytes is the largest custom icon; icons are shown small.
const maxIconBytes = 256 << 10

// iconTypes are the image types custom icons can be.
var iconTypes = map[string]string{
	"image/png": ".png", "image/svg+xml": ".svg", "image/webp": ".webp", "image/jpeg": ".jpg",
}

func (s *Server) uploadIcon(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxIconBytes+64<<10)
	if err := r.ParseMultipartForm(maxIconBytes); err != nil {
		writeMsg(w, http.StatusRequestEntityTooLarge, "server.iconTooLarge", i18n.Args{"kb": maxIconBytes >> 10})
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeMsg(w, http.StatusBadRequest, "server.uploadNoFile", nil)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxIconBytes+1))
	if err != nil {
		fail(w, err)
		return
	}
	if len(data) > maxIconBytes {
		writeMsg(w, http.StatusRequestEntityTooLarge, "server.iconTooLarge", i18n.Args{"kb": maxIconBytes >> 10})
		return
	}
	mimeType := detectType(data, hdr.Filename)
	if mimeType == "text/xml" || mimeType == "application/xml" || bytes.Contains(data[:min(len(data), 512)], []byte("<svg")) {
		mimeType = "image/svg+xml"
	}
	ext, ok := iconTypes[mimeType]
	if !ok {
		writeMsg(w, http.StatusBadRequest, "server.iconType", nil)
		return
	}
	name, _, err := s.files.Save(bytes.NewReader(data), ext)
	if err != nil {
		fail(w, err)
		return
	}
	icon, err := s.store.AddIcon(r.Context(), household(r), name, mimeType)
	if err != nil {
		s.files.Remove(name)
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, icon)
}

func (s *Server) listIcons(w http.ResponseWriter, r *http.Request) {
	icons, err := s.store.ListIcons(r.Context(), household(r))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, icons)
}

// icon serves a custom icon. SVGs are shown as images, where their scripts
// don't run, and the policy stops them running if opened directly.
func (s *Server) icon(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	i, err := s.store.GetIcon(r.Context(), household(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	s.serveFile(w, r, i.File, "icon-"+strconv.FormatInt(i.ID, 10)+iconTypes[i.MimeType], i.MimeType, false)
}

func (s *Server) deleteIcon(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	i, err := s.store.DeleteIcon(r.Context(), household(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	s.files.Remove(i.File)
	w.WriteHeader(http.StatusNoContent)
}
