package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fhavrlent/wherehouse/internal/i18n"
	"github.com/fhavrlent/wherehouse/internal/paperless"
	"github.com/fhavrlent/wherehouse/internal/store"
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
// links on: by, who changed something, and when borrow is set and
// Paperless refuses them, whoever linked the document, including those
// whose links were just removed (ref.Linkers), then whoever wrote its
// link, so a link to a thing that is gone still gets corrected. Only those
// have shown they may reach the document; nobody else's connection is ever
// used for it. Restoring a backup doesn't borrow, as its document
// IDs come from a file. A change made with an API token never uses the
// server's own token, as with userPaperless.
func (s *Server) syncBacklink(ctx context.Context, by actor, ref store.DocumentRef, borrow bool) error {
	doc := ref.Doc
	// One update per document at a time, reading the latest state once it's
	// its turn, so an older update can't finish last and win.
	lock, _ := s.docLocks.LoadOrStore(doc, new(sync.Mutex))
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	candidates := []int64{by.uid}
	if borrow {
		linkers, err := s.store.DocumentLinkers(ctx, doc)
		if err != nil {
			return err
		}
		writer, err := s.store.BacklinkWriter(ctx, doc)
		if err != nil {
			return err
		}
		candidates = slices.Concat(candidates, ref.Linkers, linkers, writer)
	}
	var refused error
	tried := map[int64]bool{}
	for _, id := range candidates {
		if tried[id] {
			continue
		}
		tried[id] = true
		pc, err := s.backlinkClient(ctx, id, by.token)
		if err != nil {
			return err
		}
		if pc == nil {
			continue
		}
		err = s.writeBacklink(ctx, pc, id, doc)
		var pe *paperless.Error
		if errors.As(err, &pe) && (pe.Status == http.StatusUnauthorized || pe.Status == http.StatusForbidden || pe.Status == http.StatusNotFound) {
			// This connection may not reach or change the document; the next
			// one may.
			refused = err
			continue
		}
		return err
	}
	return refused
}

// writeBacklink writes a document's link with pc, the connection of uid.
func (s *Server) writeBacklink(ctx context.Context, pc *paperless.Client, uid, doc int64) error {
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
	fields, changed := setLinkField(current, field, want, ourLink(app, doc, things))
	if !changed {
		// Nothing written, so nothing shows this connection may write: who
		// wrote the link stays as it was.
		return nil
	}
	if err := pc.SetDocumentCustomFields(ctx, doc, fields); err != nil {
		return err
	}
	if want == "" {
		uid = 0
	}
	return s.store.SetBacklinkWriter(ctx, doc, uid)
}

// backlinkClient is uid's Paperless connection if they turned links on;
// with token, a change made with an API token, never the server's token.
func (s *Server) backlinkClient(ctx context.Context, uid int64, token bool) (*paperless.Client, error) {
	st, err := s.store.Settings(ctx, uid)
	if err != nil || !st.PaperlessBacklinks {
		return nil, err
	}
	admin := false
	if !token {
		if admin, err = s.store.IsAdmin(ctx, uid); err != nil {
			return nil, err
		}
	}
	return s.paperlessFor(st, admin), nil
}

// actor is who made a change that updates links back: a person, and
// whether through an API token.
type actor struct {
	uid   int64
	token bool
}

func actorOf(r *http.Request) actor { return actor{uid: userID(r), token: isToken(r)} }

// ourLink tells a link Wherehouse wrote in a document's field from one
// someone typed in. Links written while Wherehouse had another address
// count too: the document's own /d/ link, the only one earlier versions
// wrote, and a link to a thing the document is on.
func ourLink(app string, doc int64, things []int64) func(string) bool {
	return func(v string) bool {
		if strings.HasPrefix(v, app+"/i/") || strings.HasPrefix(v, app+"/d/") {
			return true
		}
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
			return false
		}
		if u.Path == fmt.Sprintf("/d/%d", doc) {
			return true
		}
		for _, t := range things {
			if u.Path == fmt.Sprintf("/i/%d", t) {
				return true
			}
		}
		return false
	}
}

// setLinkField sets field id to want, or removes it when want is empty,
// unless it holds a value that isn't ours. It reports whether anything
// changed.
func setLinkField(fields []paperless.CustomField, id int64, want string, ours func(string) bool) ([]paperless.CustomField, bool) {
	for i, f := range fields {
		if f.Field != id {
			continue
		}
		cur, isText := f.Value.(string)
		if (f.Value != nil && !isText) || (cur != "" && !ours(cur)) {
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
// borrow is as for syncBacklink.
func (s *Server) syncBacklinksLater(by actor, borrow bool, docs ...store.DocumentRef) {
	if len(docs) == 0 || s.cfg.Paperless.URL == "" {
		return
	}
	s.backlinks.Add(1)
	go func() {
		defer s.backlinks.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		for _, d := range docs {
			if err := s.syncBacklink(ctx, by, d, borrow); err != nil {
				slog.Warn("update link in paperless", "doc", d.Doc, "err", err)
			}
		}
	}()
}

// Wait waits for links being written in the background, or until ctx ends.
func (s *Server) Wait(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		s.backlinks.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
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
		if err := s.syncBacklink(ctx, actorOf(r), store.DocumentRef{Doc: d}, false); err != nil {
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

// docRefs are documents with nobody's link to them just removed.
func docRefs(ids ...int64) []store.DocumentRef {
	out := make([]store.DocumentRef, len(ids))
	for i, id := range ids {
		out[i] = store.DocumentRef{Doc: id}
	}
	return out
}
