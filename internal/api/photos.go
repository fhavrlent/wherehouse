package api

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/fhavrlent/wherehouse/internal/i18n"
	"github.com/fhavrlent/wherehouse/internal/store"
	thumbnail "github.com/fhavrlent/wherehouse/internal/thumb"
)

// uploadPhotos adds the photos in the form's "file" fields to a thing.
func (s *Server) uploadPhotos(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	limit := s.cfg.MaxUploadMB << 20
	r.Body = http.MaxBytesReader(w, r.Body, limit+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeMsg(w, http.StatusRequestEntityTooLarge, "server.uploadTooLarge", i18n.Args{"mb": s.cfg.MaxUploadMB})
		return
	}
	defer r.MultipartForm.RemoveAll()
	headers := r.MultipartForm.File["file"]
	if len(headers) == 0 {
		writeMsg(w, http.StatusBadRequest, "server.uploadNoFile", nil)
		return
	}
	out := []store.Photo{}
	for _, hdr := range headers {
		file, err := hdr.Open()
		if err != nil {
			fail(w, err)
			return
		}
		head := make([]byte, 512)
		n, _ := io.ReadFull(file, head)
		mimeType := detectType(head[:n], hdr.Filename)
		if !strings.HasPrefix(mimeType, "image/") {
			file.Close()
			writeMsg(w, http.StatusBadRequest, "server.photoNotImage", nil)
			return
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			file.Close()
			fail(w, err)
			return
		}
		name, size, err := s.files.Save(file, strings.ToLower(filepath.Ext(cleanFilename(hdr.Filename))))
		file.Close()
		if err != nil {
			fail(w, err)
			return
		}
		p := &store.Photo{ThingID: id, File: name, MimeType: mimeType, Size: size}
		if err := s.store.AddPhoto(r.Context(), household(r), userID(r), p); err != nil {
			s.files.Remove(name)
			fail(w, err)
			return
		}
		out = append(out, *p)
	}
	writeJSON(w, http.StatusCreated, out)
}

// photo serves a photo; with ?size=thumb, a small preview of it.
func (s *Server) photo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	p, err := s.store.GetPhoto(r.Context(), household(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	// Photo IDs aren't reused, and a photo never changes under its ID.
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; frame-ancestors 'self'")
	if r.URL.Query().Get("size") == "thumb" && thumbnail.Supported(p.MimeType) && s.serveThumb(w, r, p.File) {
		return
	}
	s.serveFile(w, r, p.File, "photo"+filepath.Ext(p.File), p.MimeType, false)
}

func (s *Server) makeCover(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.MakeCover(r.Context(), household(r), id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deletePhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	p, err := s.store.DeletePhoto(r.Context(), household(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	s.files.Remove(p.File)
	w.WriteHeader(http.StatusNoContent)
}
