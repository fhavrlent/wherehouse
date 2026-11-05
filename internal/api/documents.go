package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/fhavrlent/wherehouse/internal/i18n"
	"github.com/fhavrlent/wherehouse/internal/paperless"
	"github.com/fhavrlent/wherehouse/internal/store"
)

// paperlessFor returns the Paperless client a user may use, or nil.
//
// The server's own token (PAPERLESS_TOKEN) is only ever used for admins:
// handing it to everyone would let anyone read every document it can see.
// Everyone else brings their own token, so Paperless's permissions apply.
func (s *Server) paperlessFor(st *store.Settings, admin bool) *paperless.Client {
	url := s.cfg.Paperless.URL
	switch {
	case url == "":
		return nil
	case st.PaperlessToken != "":
		return paperless.New(url, st.PaperlessToken)
	case admin && s.cfg.Paperless.Token != "":
		return paperless.New(url, s.cfg.Paperless.Token)
	}
	return nil
}

func (s *Server) userPaperless(ctx context.Context, uid int64) (*paperless.Client, error) {
	st, err := s.store.Settings(ctx, uid)
	if err != nil {
		return nil, err
	}
	admin, err := s.store.IsAdmin(ctx, uid)
	if err != nil {
		return nil, err
	}
	return s.paperlessFor(st, admin), nil
}

func (s *Server) uploadAttachment(w http.ResponseWriter, r *http.Request) {
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
	kind := r.FormValue("kind")
	if !store.ValidKind(kind) {
		kind = "other"
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeMsg(w, http.StatusBadRequest, "server.uploadNoFile", nil)
		return
	}
	defer file.Close()
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		fail(w, err)
		return
	}
	filename := cleanFilename(hdr.Filename)
	name, size, err := s.files.Save(file, strings.ToLower(filepath.Ext(filename)))
	if err != nil {
		fail(w, err)
		return
	}
	uid := userID(r)
	a := &store.Attachment{ThingID: id, Kind: kind, Storage: store.StorageLocal, Filename: filename,
		MimeType: detectType(head[:n], filename), Size: size, File: name, AddedBy: &uid}
	if err := s.store.AddAttachment(r.Context(), household(r), a); err != nil {
		s.files.Remove(name)
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

// linkPaperless adds documents from the user's Paperless to a thing.
func (s *Server) linkPaperless(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		DocumentIDs []int64 `json:"documentIds"`
		Kind        string  `json:"kind"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !store.ValidKind(in.Kind) {
		in.Kind = "other"
	}
	if len(in.DocumentIDs) == 0 {
		writeMsg(w, http.StatusBadRequest, "server.linkNeedsDocument", nil)
		return
	}
	uid := userID(r)
	pc, err := s.userPaperless(r.Context(), uid)
	if err != nil {
		fail(w, err)
		return
	}
	if pc == nil {
		writeMsg(w, http.StatusBadRequest, "server.paperlessNotConfigured", nil)
		return
	}
	out := []*store.Attachment{}
	for _, docID := range in.DocumentIDs {
		doc, err := pc.Document(r.Context(), docID)
		if err != nil {
			writeMsg(w, http.StatusBadGateway, "server.paperlessDocument", i18n.Args{"id": docID, "detail": err.Error()})
			return
		}
		filename := doc.OriginalFileName
		if doc.Title != "" {
			filename = doc.Title + filepath.Ext(doc.OriginalFileName)
		}
		a := &store.Attachment{ThingID: id, Kind: in.Kind, Storage: store.StoragePaperless,
			Filename: cleanFilename(filename), MimeType: doc.MimeType, PaperlessDocID: &doc.ID, AddedBy: &uid}
		if err := s.store.AddAttachment(r.Context(), household(r), a); err != nil {
			fail(w, err)
			return
		}
		out = append(out, a)
	}
	if st, err := s.store.Settings(r.Context(), uid); err == nil && st.PaperlessBacklinks {
		for _, id := range in.DocumentIDs {
			go s.linkBack(pc, id)
		}
	}
	writeJSON(w, http.StatusCreated, out)
}

// linkBack fills the document's Wherehouse URL field with the page of the
// things it is on, unless someone put something else there. The page lists
// whatever the document is on at the time, so the link never needs
// changing.
func (s *Server) linkBack(pc *paperless.Client, doc int64) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	want := fmt.Sprintf("%s/d/%d", strings.TrimSuffix(s.cfg.AppURL.String(), "/"), doc)
	field, err := pc.CustomFieldID(ctx, s.cfg.Paperless.Field, "url", true)
	if err != nil {
		slog.Warn("paperless link back: field", "doc", doc, "err", err)
		return
	}
	fields, err := pc.DocumentCustomFields(ctx, doc)
	if err != nil {
		slog.Warn("paperless link back: read", "doc", doc, "err", err)
		return
	}
	for _, f := range fields {
		if f.Field == field {
			if v, _ := f.Value.(string); v != "" {
				return
			}
		}
	}
	fields = append(fields, paperless.CustomField{Field: field, Value: want})
	if err := pc.SetDocumentCustomFields(ctx, doc, fields); err != nil {
		slog.Warn("paperless link back: write", "doc", doc, "err", err)
	}
}

// documentThings lists the things a Paperless document is on, for the
// page its link back opens.
func (s *Server) documentThings(w http.ResponseWriter, r *http.Request) {
	doc, ok := pathID(w, r, "doc")
	if !ok {
		return
	}
	things, err := s.store.ThingsWithDocument(r.Context(), household(r), doc)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, things)
}

func (s *Server) searchPaperless(w http.ResponseWriter, r *http.Request) {
	pc, err := s.userPaperless(r.Context(), userID(r))
	if err != nil {
		fail(w, err)
		return
	}
	if pc == nil {
		writeMsg(w, http.StatusBadRequest, "server.paperlessNotConfigured", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	docs, err := pc.Search(ctx, r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, 200, docs)
}

// attachmentFile serves a document: a stored file, or a linked Paperless
// document through the Paperless connection of whoever linked it, so the
// rest of the household can open it without access to anything else there.
func (s *Server) attachmentFile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	a, err := s.store.GetAttachment(r.Context(), household(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	download := r.URL.Query().Get("download") == "1"
	w.Header().Set("Cache-Control", "private, max-age=3600")
	// A strict default-src blocks Chrome's built-in PDF viewer, so PDFs only
	// get frame-ancestors; other types never render as active content.
	if a.MimeType == "application/pdf" || a.Storage == store.StoragePaperless {
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
	} else {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; frame-ancestors 'self'")
	}
	if a.Storage == store.StorageLocal {
		s.serveFile(w, r, a.File, a.Filename, a.MimeType, download)
		return
	}
	owner := userID(r)
	if a.AddedBy != nil {
		owner = *a.AddedBy
	}
	if a.PaperlessDocID == nil || s.cfg.Paperless.URL == "" {
		writeMsg(w, http.StatusNotFound, "server.paperlessNotConfigured", nil)
		return
	}
	// Without a token here, or if Paperless can't be reached from here, the
	// document opens in Paperless itself, which asks for its own sign-in.
	inPaperless := func() {
		http.Redirect(w, r, fmt.Sprintf("%s/documents/%d/details", s.cfg.Paperless.PublicURL, *a.PaperlessDocID), http.StatusFound)
	}
	pc, err := s.userPaperless(r.Context(), owner)
	if err != nil || pc == nil {
		inPaperless()
		return
	}
	variant := "preview"
	if download {
		variant = "download"
	}
	resp, err := pc.Stream(r.Context(), *a.PaperlessDocID, variant)
	if paperless.IsNotFound(err) {
		writeMsg(w, http.StatusNotFound, "server.paperlessGone", nil)
		return
	}
	if err != nil {
		slog.Warn("paperless document", "doc", *a.PaperlessDocID, "err", err)
		inPaperless()
		return
	}
	defer resp.Body.Close()
	ct := strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0]
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	filename := a.Filename
	// Paperless serves the archived PDF for previews; keep the name sensible.
	if variant == "preview" && ct == "application/pdf" && !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".pdf"
	}
	setDisposition(w, filename, ct, download)
	io.Copy(w, resp.Body)
}

func (s *Server) updateAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Kind     string `json:"kind"`
		Filename string `json:"filename"`
	}
	if !decode(w, r, &in) {
		return
	}
	a, err := s.store.UpdateAttachment(r.Context(), household(r), id, in.Kind, cleanName(in.Filename))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, a)
}

// cleanName is cleanFilename, but "" stays empty so it can be refused.
func cleanName(name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	return cleanFilename(strings.TrimSpace(name))
}

func (s *Server) deleteAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	a, err := s.store.DeleteAttachment(r.Context(), household(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	if a.File != "" {
		s.files.Remove(a.File)
	}
	w.WriteHeader(http.StatusNoContent)
}
