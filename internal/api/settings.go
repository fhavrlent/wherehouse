package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/fhavrlent/wherehouse/internal/auth"
	"github.com/fhavrlent/wherehouse/internal/i18n"
	"github.com/fhavrlent/wherehouse/internal/notify"
	"github.com/fhavrlent/wherehouse/internal/store"
)

type settingsResponse struct {
	*store.Settings
	IsAdmin   bool     `json:"isAdmin"`
	Languages []string `json:"languages"`
	Timezone  string   `json:"timezone"`
	// NotifyAllowedHosts limits self-hosted notification services for non-admins.
	NotifyAllowedHosts []string `json:"notifyAllowedHosts"`
	// PaperlessURL is where the browser opens the server's Paperless, "" if none.
	PaperlessURL      string `json:"paperlessUrl"`
	HasPaperlessToken bool   `json:"hasPaperlessToken"`
	// PaperlessServerToken: an admin may use the server's token.
	PaperlessServerToken bool `json:"paperlessServerToken"`
	PaperlessEnabled     bool `json:"paperlessEnabled"`
	// PaperlessField is the custom field documents link back with.
	PaperlessField string `json:"paperlessField"`
}

func (s *Server) settingsResponse(st *store.Settings, admin bool) settingsResponse {
	return settingsResponse{
		Settings:             st,
		IsAdmin:              admin,
		Languages:            i18n.Languages(),
		Timezone:             s.cfg.Location.String(),
		NotifyAllowedHosts:   s.cfg.NotifyAllowedHosts,
		PaperlessURL:         s.cfg.Paperless.PublicURL,
		HasPaperlessToken:    st.PaperlessToken != "",
		PaperlessServerToken: admin && s.cfg.Paperless.URL != "" && s.cfg.Paperless.Token != "",
		PaperlessEnabled:     s.paperlessFor(st, admin) != nil,
		PaperlessField:       s.cfg.Paperless.Field,
	}
}

func isAdmin(r *http.Request) bool { return auth.UserFrom(r.Context()).IsAdmin }

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Settings(r.Context(), userID(r))
	if err != nil {
		fail(w, err)
		return
	}
	// Remember the browser's language: reminders for people who follow it
	// are sent in it.
	if lang := i18n.Match(r.Header.Get("Accept-Language")); lang != "" && lang != st.BrowserLanguage {
		if err := s.store.SetBrowserLanguage(r.Context(), userID(r), lang); err != nil {
			fail(w, err)
			return
		}
		st.BrowserLanguage = lang
	}
	writeJSON(w, 200, s.settingsResponse(st, isAdmin(r)))
}

type settingsInput struct {
	Theme      string `json:"theme"`
	Language   string `json:"language"` // "" follows the browser
	NotifyURLs string `json:"notifyUrls"`
	NotifyDays string `json:"notifyDays"`
	// PaperlessToken nil keeps the stored token.
	PaperlessToken *string `json:"paperlessToken"`
	// PaperlessBacklinks nil keeps the setting.
	PaperlessBacklinks *bool `json:"paperlessBacklinks"`
}

func (in *settingsInput) apply(st *store.Settings, admin bool, policy notify.Policy) error {
	msg := func(key string) error { return i18n.NewError("server."+key, nil) }
	switch {
	case in.Theme != "system" && in.Theme != "light" && in.Theme != "dark":
		return msg("theme")
	case in.Language != "" && !i18n.Supported(in.Language):
		return msg("language")
	}
	if _, err := notify.ParseDays(in.NotifyDays); err != nil {
		return err
	}
	urls := notify.ParseURLs(in.NotifyURLs)
	if err := notify.Validate(urls); err != nil {
		return i18n.NewError("server.notifyUrl", i18n.Args{"detail": err.Error()})
	}
	if err := policy.CheckAll(urls, admin); err != nil {
		return err
	}
	st.Theme, st.Language = in.Theme, in.Language
	st.NotifyURLs, st.NotifyDays = strings.TrimSpace(in.NotifyURLs), strings.TrimSpace(in.NotifyDays)
	if in.PaperlessToken != nil {
		st.PaperlessToken = strings.TrimSpace(*in.PaperlessToken)
	}
	if in.PaperlessBacklinks != nil {
		st.PaperlessBacklinks = *in.PaperlessBacklinks
	}
	return nil
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var in settingsInput
	if !decode(w, r, &in) {
		return
	}
	st, err := s.store.Settings(r.Context(), userID(r))
	if err != nil {
		fail(w, err)
		return
	}
	if err := in.apply(st, isAdmin(r), s.notifyPolicy()); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.SaveSettings(r.Context(), userID(r), st); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, s.settingsResponse(st, isAdmin(r)))
}

func (s *Server) notifyPolicy() notify.Policy {
	return notify.Policy{AllowedHosts: s.cfg.NotifyAllowedHosts}
}

func (s *Server) testNotification(w http.ResponseWriter, r *http.Request) {
	var in struct {
		NotifyURLs string `json:"notifyUrls"`
	}
	if !decode(w, r, &in) {
		return
	}
	urls := notify.ParseURLs(in.NotifyURLs)
	if len(urls) == 0 {
		writeMsg(w, http.StatusBadRequest, "server.notifyNeedsUrl", nil)
		return
	}
	if err := s.notifyPolicy().CheckAll(urls, isAdmin(r)); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	st, err := s.store.Settings(r.Context(), userID(r))
	if err != nil {
		fail(w, err)
		return
	}
	lang := st.EffectiveLanguage()
	if err := notify.Send(urls, i18n.T(lang, "server.testTitle", nil), i18n.T(lang, "server.testBody", nil)); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// testPaperless checks a token before it is saved; no token tests the
// stored one.
func (s *Server) testPaperless(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token *string `json:"token"`
	}
	if !decode(w, r, &in) {
		return
	}
	if s.cfg.Paperless.URL == "" {
		writeMsg(w, http.StatusBadRequest, "server.paperlessNotSetUp", nil)
		return
	}
	st, err := s.store.Settings(r.Context(), userID(r))
	if err != nil {
		fail(w, err)
		return
	}
	probe := *st
	if in.Token != nil {
		probe.PaperlessToken = strings.TrimSpace(*in.Token)
	}
	pc := s.paperlessFor(&probe, isAdmin(r))
	if pc == nil {
		writeMsg(w, http.StatusBadRequest, "server.paperlessNeedsYourToken", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	info, err := pc.Check(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, 200, info)
}
