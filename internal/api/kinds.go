package api

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	// Checked here rather than in the store, so a backup or import with two
	// kinds of one name still comes in.
	if err := s.store.CheckKindName(r.Context(), household(r), 0, in.Name); err != nil {
		fail(w, err)
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
	// A kind keeps a name it already has, so one of two kinds named alike can
	// still get a new icon; a new name must be free.
	if err := s.checkRename(r, id, in.Name); err != nil {
		fail(w, err)
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
	var moveTo int64
	if v := r.URL.Query().Get("moveTo"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			writeMsg(w, http.StatusBadRequest, "server.kindMissing", nil)
			return
		}
		moveTo = n
	}
	if err := s.store.DeleteKind(r.Context(), household(r), id, moveTo); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) checkRename(r *http.Request, id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	kinds, err := s.store.ListKinds(r.Context(), household(r))
	if err != nil {
		return err
	}
	for _, k := range kinds {
		if k.ID == id && strings.EqualFold(k.Name, name) {
			return nil
		}
	}
	return s.store.CheckKindName(r.Context(), household(r), id, name)
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
