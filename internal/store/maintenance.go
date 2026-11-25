package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fhavrlent/wherehouse/internal/i18n"
)

// Maintenance is an entry in an item's maintenance log: something done to
// it (DoneOn), something to do (NextDue), or both.
type Maintenance struct {
	ID        int64    `json:"id"`
	ThingID   int64    `json:"thingId"`
	Title     string   `json:"title"`
	DoneOn    *string  `json:"doneOn"`
	NextDue   *string  `json:"nextDue"`
	Notes     string   `json:"notes"`
	Cost      *float64 `json:"cost"`
	Repeat    *Repeat  `json:"repeat"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
	// ThingName and Path, for lists of what is due.
	ThingName string `json:"thingName,omitempty"`
	Path      []Ref  `json:"path,omitempty"`
}

// Repeat is how often a task comes back: every Every weeks or months.
type Repeat struct {
	Every int    `json:"every"`
	Unit  string `json:"unit"`
}

const (
	RepeatWeek  = "week"
	RepeatMonth = "month"
)

func (r *Repeat) validate() error {
	if r == nil {
		return nil
	}
	max := map[string]int{RepeatWeek: 520, RepeatMonth: 120}[r.Unit]
	if max == 0 || r.Every < 1 || r.Every > max {
		return invalid("maintenanceRepeat", nil)
	}
	return nil
}

// After gives the date r after an ISO date. A month later than the 31st is
// the last day of a shorter month, not a day in the month after.
func (r Repeat) After(date string) (string, error) {
	d, err := time.Parse(DateLayout, date)
	if err != nil {
		return "", err
	}
	if r.Unit == RepeatWeek {
		return d.AddDate(0, 0, 7*r.Every).Format(DateLayout), nil
	}
	first := time.Date(d.Year(), d.Month()+time.Month(r.Every), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(d.Day(), last)-1).Format(DateLayout), nil
}

// RepeatField is a Repeat in a request that remembers whether it was sent at
// all, so an app that doesn't know about repeats keeps them when it changes
// an entry. null, or not sending it with a new entry, means no repeat.
type RepeatField struct {
	Set   bool
	Value *Repeat
}

func (f *RepeatField) UnmarshalJSON(b []byte) error {
	f.Set = true
	return json.Unmarshal(b, &f.Value)
}

func (f RepeatField) MarshalJSON() ([]byte, error) { return json.Marshal(f.Value) }

type MaintenanceInput struct {
	Title   string      `json:"title"`
	DoneOn  *string     `json:"doneOn"`
	NextDue *string     `json:"nextDue"`
	Notes   string      `json:"notes"`
	Cost    *float64    `json:"cost"`
	Repeat  RepeatField `json:"repeat"`
}

func (in *MaintenanceInput) normalize() error {
	in.Title = strings.TrimSpace(in.Title)
	in.Notes = strings.TrimSpace(in.Notes)
	for _, d := range []**string{&in.DoneOn, &in.NextDue} {
		if err := validDate(d); err != nil {
			return err
		}
	}
	switch {
	case in.Title == "":
		return invalid("maintenanceTitle", nil)
	case utf8.RuneCountInString(in.Title) > MaxName:
		return invalid("nameLong", i18n.Args{"max": MaxName})
	case utf8.RuneCountInString(in.Notes) > MaxText:
		return invalid("textLong", i18n.Args{"max": MaxText})
	case in.DoneOn == nil && in.NextDue == nil:
		return invalid("maintenanceWhen", nil)
	case in.Cost != nil && *in.Cost < 0:
		return invalid("negativeCost", nil)
	}
	return in.Repeat.Value.validate()
}

func repeatArgs(r *Repeat) (any, any) {
	if r == nil {
		return nil, nil
	}
	return r.Every, r.Unit
}

const maintenanceColumns = `m.id, m.thing_id, m.title, m.done_on, m.next_due, m.notes, m.cost, m.repeat_every, m.repeat_unit, m.created_at, m.updated_at`

func scanMaintenance(sc interface{ Scan(...any) error }, extra ...any) (*Maintenance, error) {
	var m Maintenance
	var done, due sql.NullString
	var cost sql.NullFloat64
	var every sql.NullInt64
	var unit sql.NullString
	dest := append([]any{&m.ID, &m.ThingID, &m.Title, &done, &due, &m.Notes, &cost, &every, &unit, &m.CreatedAt, &m.UpdatedAt}, extra...)
	if err := sc.Scan(dest...); err != nil {
		return nil, err
	}
	if done.Valid {
		m.DoneOn = &done.String
	}
	if due.Valid {
		m.NextDue = &due.String
	}
	if cost.Valid {
		m.Cost = &cost.Float64
	}
	if every.Valid && unit.Valid {
		m.Repeat = &Repeat{Every: int(every.Int64), Unit: unit.String}
	}
	return &m, nil
}

// maintenanceOf lists an item's log: what is due first, then the latest done.
func maintenanceOf(ctx context.Context, q queryer, thing int64) ([]Maintenance, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+maintenanceColumns+` FROM maintenance m WHERE m.thing_id = ?
		ORDER BY m.next_due IS NULL, m.next_due, m.done_on DESC, m.id DESC`, thing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Maintenance{}
	for rows.Next() {
		m, err := scanMaintenance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) AddMaintenance(ctx context.Context, hh, uid, thing int64, in MaintenanceInput) (*Maintenance, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	var id int64
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := owned(ctx, tx, hh, thing); err != nil {
			return err
		}
		var err error
		if id, err = insertMaintenance(ctx, tx, uid, thing, in); err != nil {
			return err
		}
		return touch(ctx, tx, thing)
	})
	if err != nil {
		return nil, err
	}
	return s.GetMaintenance(ctx, hh, id)
}

func insertMaintenance(ctx context.Context, tx *sql.Tx, uid, thing int64, in MaintenanceInput) (int64, error) {
	every, unit := repeatArgs(in.Repeat.Value)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO maintenance (thing_id, title, done_on, next_due, notes, cost, repeat_every, repeat_unit, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		thing, in.Title, in.DoneOn, in.NextDue, in.Notes, in.Cost, every, unit, uid, now(), now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) GetMaintenance(ctx context.Context, hh, id int64) (*Maintenance, error) {
	m, err := scanMaintenance(s.db.QueryRowContext(ctx, `SELECT `+maintenanceColumns+` FROM maintenance m
		JOIN things t ON t.id = m.thing_id WHERE m.id = ? AND t.household_id = ?`, id, hh))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (s *Store) UpdateMaintenance(ctx context.Context, hh, id int64, in MaintenanceInput) (*Maintenance, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	m, err := s.GetMaintenance(ctx, hh, id)
	if err != nil {
		return nil, err
	}
	if !in.Repeat.Set {
		in.Repeat.Value = m.Repeat
	}
	every, unit := repeatArgs(in.Repeat.Value)
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			UPDATE maintenance SET title = ?, done_on = ?, next_due = ?, notes = ?, cost = ?, repeat_every = ?, repeat_unit = ?, updated_at = ? WHERE id = ?`,
			in.Title, in.DoneOn, in.NextDue, in.Notes, in.Cost, every, unit, now(), id); err != nil {
			return err
		}
		// Apps reading the changes see it as a change to its thing.
		return touch(ctx, tx, m.ThingID)
	})
	if err != nil {
		return nil, err
	}
	return s.GetMaintenance(ctx, hh, id)
}

func (s *Store) DeleteMaintenance(ctx context.Context, hh, id int64) error {
	m, err := s.GetMaintenance(ctx, hh, id)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM maintenance WHERE id = ?`, id); err != nil {
			return err
		}
		return touch(ctx, tx, m.ThingID)
	})
}

// DoneInput records a due task as done.
type DoneInput struct {
	DoneOn string   `json:"doneOn"`
	Cost   *float64 `json:"cost"`
	// The repeat for the next entry; when not sent, the entry's own.
	Repeat RepeatField `json:"repeat"`
}

// CompleteMaintenance logs a due entry as done on a day and, if it repeats,
// adds the next one that long after. The done entry stays in the log as
// history, without a due date or repeat; the repeat moves to the next entry.
// An entry that was already done once keeps that date, and this time gets an
// entry of its own. next is nil when the task doesn't repeat.
func (s *Store) CompleteMaintenance(ctx context.Context, hh, uid, id int64, in DoneInput) (done, next *Maintenance, err error) {
	doneOn := &in.DoneOn
	if err := validDate(&doneOn); err != nil {
		return nil, nil, err
	}
	if doneOn == nil {
		return nil, nil, invalid("maintenanceDoneOn", nil)
	}
	if in.Cost != nil && *in.Cost < 0 {
		return nil, nil, invalid("negativeCost", nil)
	}
	m, err := s.GetMaintenance(ctx, hh, id)
	if err != nil {
		return nil, nil, err
	}
	repeat := m.Repeat
	if in.Repeat.Set {
		repeat = in.Repeat.Value
	}
	if err := repeat.validate(); err != nil {
		return nil, nil, err
	}
	cost := m.Cost
	if in.Cost != nil {
		cost = in.Cost
	}
	if m.NextDue == nil {
		return nil, nil, invalid("maintenanceNotDue", nil)
	}
	doneID, nextID := id, int64(0)
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if m.DoneOn == nil {
			if _, err := tx.ExecContext(ctx, `
				UPDATE maintenance SET done_on = ?, next_due = NULL, cost = ?, repeat_every = NULL, repeat_unit = NULL, updated_at = ? WHERE id = ?`,
				*doneOn, cost, now(), id); err != nil {
				return err
			}
		} else {
			// It was done before and due again: that earlier time stays as it
			// was, and this time is an entry of its own.
			if _, err := tx.ExecContext(ctx, `
				UPDATE maintenance SET next_due = NULL, repeat_every = NULL, repeat_unit = NULL, updated_at = ? WHERE id = ?`,
				now(), id); err != nil {
				return err
			}
			var err error
			if doneID, err = insertMaintenance(ctx, tx, uid, m.ThingID, MaintenanceInput{
				Title: m.Title, DoneOn: doneOn, Notes: m.Notes, Cost: in.Cost,
			}); err != nil {
				return err
			}
		}
		if repeat != nil {
			due, err := repeat.After(*doneOn)
			if err != nil {
				return err
			}
			if nextID, err = insertMaintenance(ctx, tx, uid, m.ThingID, MaintenanceInput{
				Title: m.Title, NextDue: &due, Notes: m.Notes, Repeat: RepeatField{Set: true, Value: repeat},
			}); err != nil {
				return err
			}
		}
		return touch(ctx, tx, m.ThingID)
	})
	if err != nil {
		return nil, nil, err
	}
	if done, err = s.GetMaintenance(ctx, hh, doneID); err != nil {
		return nil, nil, err
	}
	if nextID != 0 {
		if next, err = s.GetMaintenance(ctx, hh, nextID); err != nil {
			return nil, nil, err
		}
	}
	return done, next, nil
}

// DueMaintenance lists entries due on or before until (a date), soonest
// first, for current things of a household.
func (s *Store) DueMaintenance(ctx context.Context, hh int64, until string) ([]Maintenance, error) {
	return s.maintenanceAcross(ctx, hh, `m.next_due IS NOT NULL AND m.next_due <= ?`, `m.next_due, m.id`, until)
}

// HouseholdMaintenance lists, for current things of a household, every
// entry with a due date, soonest first, then what was done on or after
// since, latest first.
func (s *Store) HouseholdMaintenance(ctx context.Context, hh int64, since string) ([]Maintenance, error) {
	return s.maintenanceAcross(ctx, hh, `(m.next_due IS NOT NULL OR m.done_on >= ?)`,
		`m.next_due IS NULL, m.next_due, m.done_on DESC, m.id DESC`, since)
}

func (s *Store) maintenanceAcross(ctx context.Context, hh int64, where, order string, args ...any) ([]Maintenance, error) {
	t, err := s.loadTree(ctx, s.db, hh)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+maintenanceColumns+`, t.name, t.parent_id FROM maintenance m JOIN things t ON t.id = m.thing_id
		WHERE t.household_id = ? AND `+where+` AND t.archived_at IS NULL
			AND t.is_template = 0 AND t.sold_date IS NULL
		ORDER BY `+order, append([]any{hh}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Maintenance{}
	for rows.Next() {
		var place sql.NullInt64
		var name string
		m, err := scanMaintenance(rows, &name, &place)
		if err != nil {
			return nil, err
		}
		m.ThingName = name
		var p *int64
		if place.Valid {
			p = &place.Int64
		}
		m.Path = t.path(p)
		out = append(out, *m)
	}
	return out, rows.Err()
}

// ReminderSent reports whether a user was reminded of an entry due then,
// threshold days ahead.
func (s *Store) ReminderSent(ctx context.Context, user, entry int64, due string, threshold int) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM reminders_sent WHERE user_id = ? AND maintenance_id = ? AND due = ? AND threshold_days = ?`,
		user, entry, due, threshold).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) MarkReminderSent(ctx context.Context, user, entry int64, due string, thresholds []int) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, th := range thresholds {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO reminders_sent (user_id, maintenance_id, due, threshold_days) VALUES (?, ?, ?, ?)`,
				user, entry, due, th); err != nil {
				return err
			}
		}
		return nil
	})
}
