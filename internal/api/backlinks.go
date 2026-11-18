package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fhavrlent/wherehouse/internal/i18n"
	"github.com/fhavrlent/wherehouse/internal/paperless"
)

// Links back from Paperless: people who turn them on get a URL custom field
// on the documents linked to things, which opens the thing the document is
// on, or the list of them when there are several.

// syncBacklink brings the link field of a Paperless document up to date: it
// points to the thing the document is on, or to the page listing them when
// there are several, and is removed when there is none. A value Wherehouse
// didn't write, such as one someone typed in, is left alone.
//
// The field is written with the Paperless connection of someone who turned
// links on: uid, who changed something, if they did, else whoever linked
// the document, else anyone, so a link written earlier still gets corrected.
func (s *Server) syncBacklink(ctx context.Context, uid, doc int64) error {
	// One update per document at a time, reading the latest state once it's
	// its turn, so an older update can't finish last and win.
	lock, _ := s.docLocks.LoadOrStore(doc, new(sync.Mutex))
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	pc, err := s.backlinkWriter(ctx, uid, doc)
	if err != nil || pc == nil {
		return err
	}
	things, err := s.store.DocumentThingIDs(ctx, doc)
	if err != nil {
		return err
	}
	app := strings.TrimSuffix(s.cfg.AppURL.String(), "/")
	want := ""
	switch {
	case len(things) == 1:
		want = fmt.Sprintf("%s/i/%d", app, things[0])
	case len(things) > 1:
		want = fmt.Sprintf("%s/d/%d", app, doc)
	}
	// A field is only created when there is something to put in it.
	field, err := pc.CustomFieldID(ctx, s.cfg.Paperless.Field, "url", want != "")
	if err != nil || field == 0 {
		return err
	}
	current, err := pc.DocumentCustomFields(ctx, doc)
	if err != nil {
		return err
	}
	fields, changed := setLinkField(current, field, want, app+"/i/", app+"/d/")
	if !changed {
		return nil
	}
	return pc.SetDocumentCustomFields(ctx, doc, fields)
}

// backlinkWriter is the Paperless connection to write a document's link
// with, or nil when nobody who could has links on.
func (s *Server) backlinkWriter(ctx context.Context, uid, doc int64) (*paperless.Client, error) {
	linkers, err := s.store.DocumentLinkers(ctx, doc)
	if err != nil {
		return nil, err
	}
	everyone, err := s.store.BacklinkUsers(ctx)
	if err != nil {
		return nil, err
	}
	tried := map[int64]bool{}
	for _, id := range append(append([]int64{uid}, linkers...), everyone...) {
		if tried[id] {
			continue
		}
		tried[id] = true
		st, err := s.store.Settings(ctx, id)
		if err != nil {
			return nil, err
		}
		if !st.PaperlessBacklinks {
			continue
		}
		admin, err := s.store.IsAdmin(ctx, id)
		if err != nil {
			return nil, err
		}
		if pc := s.paperlessFor(st, admin); pc != nil {
			return pc, nil
		}
	}
	return nil, nil
}

// setLinkField sets field id to want, or removes it when want is empty,
// unless it holds a value that doesn't start with one of ours. It reports
// whether anything changed.
func setLinkField(fields []paperless.CustomField, id int64, want string, ours ...string) ([]paperless.CustomField, bool) {
	for i, f := range fields {
		if f.Field != id {
			continue
		}
		cur, isText := f.Value.(string)
		mine := cur == ""
		for _, p := range ours {
			mine = mine || strings.HasPrefix(cur, p)
		}
		if (f.Value != nil && !isText) || !mine {
			return fields, false // someone else's value
		}
		if want == "" {
			return append(fields[:i:i], fields[i+1:]...), true
		}
		if cur == want {
			return fields, false
		}
		out := append([]paperless.CustomField(nil), fields...)
		out[i].Value = want
		return out, true
	}
	if want == "" {
		return fields, false
	}
	return append(fields, paperless.CustomField{Field: id, Value: want}), true
}

// syncBacklinksLater updates documents' link fields in the background, so a
// slow or refusing Paperless doesn't hold up the request that changed them.
func (s *Server) syncBacklinksLater(uid int64, docs ...int64) {
	if len(docs) == 0 || s.cfg.Paperless.URL == "" {
		return
	}
	s.backlinks.Add(1)
	go func() {
		defer s.backlinks.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		for _, d := range docs {
			if err := s.syncBacklink(ctx, uid, d); err != nil {
				slog.Warn("update link in paperless", "doc", d, "err", err)
			}
		}
	}()
}

// syncAllBacklinks updates every document the person linked, e.g. right
// after they turn links on, and reports how it went.
func (s *Server) syncAllBacklinks(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	docs, err := s.store.LinkedDocuments(r.Context(), uid)
	if err != nil {
		fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	out := struct {
		Documents int       `json:"documents"`
		Failed    int       `json:"failed"`
		Error     *apiError `json:"error,omitempty"`
	}{Documents: len(docs)}
	for _, d := range docs {
		if err := s.syncBacklink(ctx, uid, d); err != nil {
			slog.Warn("update link in paperless", "doc", d, "err", err)
			out.Failed++
			if out.Error == nil {
				out.Error = backlinkError(err)
			}
		}
	}
	writeJSON(w, 200, out)
}

// backlinkError explains why links couldn't be written; Paperless refusing
// is almost always a token that may read but not change documents.
func backlinkError(err error) *apiError {
	var pe *paperless.Error
	var fe *paperless.FieldTypeError
	switch {
	case errors.As(err, &fe):
		args := i18n.Args{"name": fe.Name}
		return &apiError{Error: i18n.T(i18n.Fallback, "server.paperlessFieldType", args), Code: "server.paperlessFieldType", Params: args}
	case errors.As(err, &pe) && (pe.Status == http.StatusForbidden || pe.Status == http.StatusUnauthorized):
		return &apiError{Error: i18n.T(i18n.Fallback, "server.paperlessCantEdit", nil), Code: "server.paperlessCantEdit"}
	}
	return &apiError{Error: err.Error()}
}
