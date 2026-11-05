package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
	// ThingName and Path, for lists of what is due.
	ThingName string `json:"thingName,omitempty"`
	Path      []Ref  `json:"path,omitempty"`
}

type MaintenanceInput struct {
	Title   string   `json:"title"`
	DoneOn  *string  `json:"doneOn"`
	NextDue *string  `json:"nextDue"`
	Notes   string   `json:"notes"`
	Cost    *float64 `json:"cost"`
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
	return nil
}

const maintenanceColumns = `m.id, m.thing_id, m.title, m.done_on, m.next_due, m.notes, m.cost, m.created_at, m.updated_at`

func scanMaintenance(sc interface{ Scan(...any) error }, extra ...any) (*Maintenance, error) {
	var m Maintenance
	var done, due sql.NullString
	var cost sql.NullFloat64
	dest := append([]any{&m.ID, &m.ThingID, &m.Title, &done, &due, &m.Notes, &cost, &m.CreatedAt, &m.UpdatedAt}, extra...)
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
		res, err := tx.ExecContext(ctx, `
			INSERT INTO maintenance (thing_id, title, done_on, next_due, notes, cost, created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			thing, in.Title, in.DoneOn, in.NextDue, in.Notes, in.Cost, uid, now(), now())
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		return touch(ctx, tx, thing)
	})
	if err != nil {
		return nil, err
	}
	return s.GetMaintenance(ctx, hh, id)
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
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			UPDATE maintenance SET title = ?, done_on = ?, next_due = ?, notes = ?, cost = ?, updated_at = ? WHERE id = ?`,
			in.Title, in.DoneOn, in.NextDue, in.Notes, in.Cost, now(), id); err != nil {
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

// DueMaintenance lists entries due on or before until (a date), soonest
// first, for current things of a household.
func (s *Store) DueMaintenance(ctx context.Context, hh int64, until string) ([]Maintenance, error) {
	t, err := s.loadTree(ctx, s.db, hh)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+maintenanceColumns+`, t.name, t.parent_id FROM maintenance m JOIN things t ON t.id = m.thing_id
		WHERE t.household_id = ? AND m.next_due IS NOT NULL AND m.next_due <= ? AND t.archived_at IS NULL
			AND t.is_template = 0 AND t.sold_date IS NULL
		ORDER BY m.next_due, m.id`, hh, until)
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
