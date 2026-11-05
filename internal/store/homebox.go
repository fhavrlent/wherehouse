package store

import (
	"context"
	"database/sql"
	"errors"
)

// Imported returns the thing a HomeBox record was imported as, or 0.
func (s *Store) Imported(ctx context.Context, hh int64, homeboxID string) (int64, error) {
	var ref int64
	err := s.db.QueryRowContext(ctx, `SELECT thing_id FROM homebox_map WHERE household_id = ? AND homebox_id = ?`, hh, homeboxID).Scan(&ref)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return ref, err
}

// MarkImported records that a HomeBox record was imported as thing.
func (s *Store) MarkImported(ctx context.Context, hh int64, homeboxID string, thing int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO homebox_map (household_id, homebox_id, thing_id) VALUES (?, ?, ?)
		ON CONFLICT (household_id, homebox_id) DO UPDATE SET thing_id = excluded.thing_id`, hh, homeboxID, thing)
	return err
}

// FromHomeBox returns the thing imported from a HomeBox entity, if any.
func (s *Store) FromHomeBox(ctx context.Context, hh int64, homeboxID string) (*Thing, error) {
	ref, err := s.Imported(ctx, hh, homeboxID)
	if err != nil {
		return nil, err
	}
	if ref == 0 {
		return nil, ErrNotFound
	}
	return s.GetThing(ctx, hh, ref)
}

// FieldByName returns the household's custom field with a name, adding it
// with typ if there is none.
func (s *Store) FieldByName(ctx context.Context, hh int64, name, typ string) (*Field, error) {
	var f Field
	err := s.db.QueryRowContext(ctx, `SELECT id, name, type, position FROM fields WHERE household_id = ? AND name = ?`, hh, name).
		Scan(&f.ID, &f.Name, &f.Type, &f.Position)
	if errors.Is(err, sql.ErrNoRows) {
		return s.CreateField(ctx, hh, name, typ)
	}
	return &f, err
}
