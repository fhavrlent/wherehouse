// Package notify sends maintenance reminders via Shoutrrr URLs (ntfy,
// Gotify, Discord, Telegram, email, Pushover, generic webhooks, ...).
package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"

	"github.com/fhavrlent/wherehouse/internal/i18n"
	"github.com/fhavrlent/wherehouse/internal/store"
)

type Notifier struct {
	store  *store.Store
	loc    *time.Location
	hour   int
	appURL string
	policy Policy
}

func New(st *store.Store, loc *time.Location, hour int, appURL string, policy Policy) *Notifier {
	return &Notifier{store: st, loc: loc, hour: hour, appURL: strings.TrimSuffix(appURL, "/"), policy: policy}
}

// ParseURLs splits the settings field into individual service URLs.
func ParseURLs(raw string) []string {
	var out []string
	for _, line := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == '\r' }) {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}

// ParseDays parses "7, 0" into a sorted, de-duplicated list.
func ParseDays(raw string) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	for _, f := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' }) {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 || n > 365 {
			return nil, i18n.NewError("server.reminderDays", i18n.Args{"value": f})
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out, nil
}

// Validate checks that every URL is understood by Shoutrrr.
func Validate(urls []string) error {
	var errs []error
	for _, u := range urls {
		if _, err := shoutrrr.CreateSender(u); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", redact(u), err))
		}
	}
	return errors.Join(errs...)
}

func redact(u string) string {
	if i := strings.Index(u, "://"); i > 0 {
		return u[:i] + "://…"
	}
	return "…"
}

func Send(urls []string, title, message string) error {
	if len(urls) == 0 {
		return errors.New("no notification URLs configured")
	}
	sender, err := shoutrrr.CreateSender(urls...)
	if err != nil {
		return err
	}
	params := types.Params{"title": title}
	var errs []error
	for _, err := range sender.Send(message, &params) {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Run checks hourly and sends reminders once the configured hour has passed.
// Each (entry, due date, threshold) fires at most once per person, so
// restarts or several checks a day never repeat one.
func (n *Notifier) Run(ctx context.Context) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		if time.Now().In(n.loc).Hour() >= n.hour {
			if err := n.CheckAll(ctx); err != nil {
				slog.Error("reminder check failed", "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (n *Notifier) CheckAll(ctx context.Context) error {
	ids, err := n.store.ListUserIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := n.checkUser(ctx, id); err != nil {
			slog.Error("reminders for user failed", "user", id, "err", err)
		}
	}
	return nil
}

func (n *Notifier) today() time.Time {
	y, m, d := time.Now().In(n.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// checkUser reminds one person of maintenance due within their reminder
// days, and of anything overdue once.
func (n *Notifier) checkUser(ctx context.Context, userID int64) error {
	st, err := n.store.Settings(ctx, userID)
	if err != nil {
		return err
	}
	admin, err := n.store.IsAdmin(ctx, userID)
	if err != nil {
		return err
	}
	urls := ParseURLs(st.NotifyURLs)
	if len(urls) == 0 {
		return nil
	}
	// Settings saved before an admin narrowed the allowed hosts may hold
	// URLs that are no longer allowed.
	if err := n.policy.CheckAll(urls, admin); err != nil {
		return nil
	}
	days, err := ParseDays(st.NotifyDays)
	if err != nil || len(days) == 0 {
		return err
	}
	today := n.today()
	until := today.AddDate(0, 0, days[len(days)-1]).Format(store.DateLayout)
	households, err := n.store.HouseholdsOf(ctx, userID)
	if err != nil {
		return err
	}
	var due []store.Maintenance
	for _, hh := range households {
		ms, err := n.store.DueMaintenance(ctx, hh, until)
		if err != nil {
			return err
		}
		due = append(due, ms...)
	}

	lang := st.EffectiveLanguage()
	var lines []string
	type mark struct {
		m          store.Maintenance
		thresholds []int
	}
	var marks []mark
	for _, m := range due {
		d, err := time.Parse(store.DateLayout, *m.NextDue)
		if err != nil {
			continue
		}
		left := int(d.Sub(today).Hours() / 24)
		// The thresholds reached by now; the nearest one not yet sent fires.
		var reached []int
		for _, th := range days {
			if left <= th {
				reached = append(reached, th)
			}
		}
		if len(reached) == 0 {
			continue
		}
		sent, err := n.store.ReminderSent(ctx, userID, m.ID, *m.NextDue, reached[0])
		if err != nil {
			return err
		}
		if sent {
			continue
		}
		var when string
		switch {
		case left < 0:
			when = i18n.T(lang, "reminder.overdue", i18n.Args{"count": -left})
		case left == 0:
			when = i18n.T(lang, "reminder.today", nil)
		default:
			when = i18n.T(lang, "reminder.inDays", i18n.Args{"count": left})
		}
		lines = append(lines, fmt.Sprintf("• %s: %s (%s)\n  %s/i/%d", m.ThingName, m.Title, when, n.appURL, m.ThingID))
		marks = append(marks, mark{m, reached})
	}
	if len(lines) == 0 {
		return nil
	}
	title := i18n.T(lang, "reminder.title", i18n.Args{"count": len(lines)})
	if err := Send(urls, title, strings.Join(lines, "\n")); err != nil {
		return err
	}
	for _, mk := range marks {
		if err := n.store.MarkReminderSent(ctx, userID, mk.m.ID, *mk.m.NextDue, mk.thresholds); err != nil {
			return err
		}
	}
	return nil
}
