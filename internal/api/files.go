package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	thumbnail "github.com/fhavrlent/wherehouse/internal/thumb"
)

// Files stores uploaded files on local disk.
type Files struct{ dir string }

// NewFiles prepares dir and checks it is writable, so a misconfigured mount
// (e.g. an NFS share with the wrong owner) fails at startup instead of on
// the first upload.
func NewFiles(dir string) (*Files, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("files directory %s: %w", dir, err)
	}
	probe, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return nil, fmt.Errorf("files directory %s is not writable (the container runs as uid 65532): %w", dir, err)
	}
	probe.Close()
	os.Remove(probe.Name())
	return &Files{dir: dir}, nil
}

// Save writes r to a new randomly named file and returns its relative name.
func (f *Files) Save(r io.Reader, ext string) (string, int64, error) {
	b := make([]byte, 16)
	rand.Read(b)
	name := hex.EncodeToString(b) + ext
	out, err := os.OpenFile(filepath.Join(f.dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", 0, err
	}
	n, err := io.Copy(out, r)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(filepath.Join(f.dir, name))
		return "", 0, err
	}
	return name, n, nil
}

func (f *Files) Open(name string) (*os.File, error) {
	if !validName(name) {
		return nil, os.ErrNotExist
	}
	return os.Open(filepath.Join(f.dir, name))
}

// Remove deletes files and their previews.
func (f *Files) Remove(names ...string) {
	for _, name := range names {
		if !validName(name) {
			continue
		}
		for _, n := range []string{name, name + thumbExt} {
			if err := os.Remove(filepath.Join(f.dir, n)); err != nil && !errors.Is(err, os.ErrNotExist) {
				slog.Warn("could not remove file", "name", n, "err", err)
			}
		}
	}
}

// thumbExt names a file's preview: the file's name plus this.
const thumbExt = ".thumb.jpg"

// writeThumb stores the preview of file name, whole or not at all.
func (f *Files) writeThumb(name string, data []byte) error {
	if !validName(name) {
		return os.ErrInvalid
	}
	tmp, err := os.CreateTemp(f.dir, ".thumb-*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), filepath.Join(f.dir, name+thumbExt))
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

func validName(name string) bool {
	return name != "" && !strings.ContainsAny(name, `/\`) && !strings.Contains(name, "..")
}

// inlineTypes are safe to render in the browser; everything else downloads.
var inlineTypes = map[string]bool{
	"application/pdf": true, "image/png": true, "image/jpeg": true, "image/gif": true,
	"image/webp": true, "image/avif": true, "text/plain": true,
}

func cleanFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." || name == "/" {
		return "file"
	}
	if len(name) > 200 {
		ext := filepath.Ext(name)
		name = name[:200-len(ext)] + ext
	}
	return name
}

// Office formats sniff as ZIP (OOXML, ODF) or as unknown binary (legacy
// formats), and the runtime image has no system MIME table, so they are
// recognised by extension.
var officeTypes = map[string]string{
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".odt":  "application/vnd.oasis.opendocument.text",
	".ods":  "application/vnd.oasis.opendocument.spreadsheet",
}

func detectType(head []byte, filename string) string {
	t := http.DetectContentType(head)
	ext := strings.ToLower(filepath.Ext(filename))
	if office, ok := officeTypes[ext]; ok && (t == "application/zip" || t == "application/octet-stream" || strings.HasPrefix(t, "text/plain")) {
		return office
	}
	if t == "application/octet-stream" || strings.HasPrefix(t, "text/plain") {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			return strings.SplitN(byExt, ";", 2)[0]
		}
	}
	return strings.SplitN(t, ";", 2)[0]
}

func setDisposition(w http.ResponseWriter, filename, mimeType string, download bool) {
	disp := "inline"
	if download || !inlineTypes[mimeType] {
		disp = "attachment"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": filename}))
}

// serveFile serves a stored file as it is.
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, name, filename, mimeType string, download bool) {
	f, err := s.files.Open(name)
	if err != nil {
		writeMsg(w, http.StatusNotFound, "server.fileMissing", nil)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	setDisposition(w, filename, mimeType, download)
	w.Header().Set("Content-Type", mimeType)
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// serveThumb serves the preview of a stored photo, making it the first
// time. It reports false when there is none to serve, e.g. for an image Go
// can't read, so the original is served instead.
func (s *Server) serveThumb(w http.ResponseWriter, r *http.Request, name string) bool {
	if !validName(name) {
		return false
	}
	serve := func() bool {
		f, err := s.files.Open(name + thumbExt)
		if err != nil {
			return false
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			return false
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Disposition", "inline")
		http.ServeContent(w, r, "", st.ModTime(), f)
		return true
	}
	if serve() {
		return true
	}
	lock, _ := s.thumbLocks.LoadOrStore(name, new(sync.Mutex))
	lock.(*sync.Mutex).Lock()
	defer func() {
		lock.(*sync.Mutex).Unlock()
		s.thumbLocks.Delete(name)
	}()
	if serve() { // made while this waited
		return true
	}
	select {
	case s.thumbSlots <- struct{}{}:
		defer func() { <-s.thumbSlots }()
	case <-r.Context().Done():
		return false
	}
	src, err := s.files.Open(name)
	if err != nil {
		return false
	}
	var buf bytes.Buffer
	err = thumbnail.Make(&buf, src)
	src.Close()
	if err != nil {
		if !errors.Is(err, thumbnail.ErrUnsupported) {
			slog.Warn("make preview", "name", name, "err", err)
		}
		return false
	}
	if err := s.files.writeThumb(name, buf.Bytes()); err != nil {
		slog.Warn("store preview", "name", name, "err", err)
		return false
	}
	return serve()
}
