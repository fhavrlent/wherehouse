package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/fhavrlent/wherehouse/internal/i18n"
)

type Photo struct {
	ID        int64  `json:"id"`
	ThingID   int64  `json:"thingId"`
	File      string `json:"-"`
	MimeType  string `json:"mimeType"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"createdAt"`
}

// MaxPhotos is the most photos one thing can have.
const MaxPhotos = 50

const photoColumns = `p.id, p.thing_id, p.file, p.mime_type, p.size, p.created_at`

func scanPhoto(sc interface{ Scan(...any) error }) (*Photo, error) {
	var p Photo
	err := sc.Scan(&p.ID, &p.ThingID, &p.File, &p.MimeType, &p.Size, &p.CreatedAt)
	return &p, err
}

// photosOf lists a thing's photos, cover first.
func (s *Store) photosOf(ctx context.Context, thing int64) ([]Photo, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+photoColumns+` FROM photos p WHERE p.thing_id = ? ORDER BY p.position, p.id`, thing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Photo{}
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// AddPhoto adds a photo to a thing, after the others.
func (s *Store) AddPhoto(ctx context.Context, hh, uid int64, p *Photo) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := owned(ctx, tx, hh, p.ThingID); err != nil {
			return err
		}
		var count, next int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(position) + 1, 0) FROM photos WHERE thing_id = ?`, p.ThingID).
			Scan(&count, &next); err != nil {
			return err
		}
		if count >= MaxPhotos {
			return invalid("tooManyPhotos", i18n.Args{"max": MaxPhotos})
		}
		p.CreatedAt = now()
		res, err := tx.ExecContext(ctx, `INSERT INTO photos (thing_id, file, mime_type, size, position, created_by, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, p.ThingID, p.File, p.MimeType, p.Size, next, uid, p.CreatedAt)
		if err != nil {
			return err
		}
		if p.ID, err = res.LastInsertId(); err != nil {
			return err
		}
		return touch(ctx, tx, p.ThingID)
	})
}

// GetPhoto returns a photo of a thing in household hh.
func (s *Store) GetPhoto(ctx context.Context, hh, id int64) (*Photo, error) {
	p, err := scanPhoto(s.db.QueryRowContext(ctx, `SELECT `+photoColumns+` FROM photos p JOIN things t ON t.id = p.thing_id
		WHERE p.id = ? AND t.household_id = ?`, id, hh))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// MakeCover moves a photo to the front.
func (s *Store) MakeCover(ctx context.Context, hh, id int64) error {
	p, err := s.GetPhoto(ctx, hh, id)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE photos SET position = position + 1 WHERE thing_id = ? AND id <> ?`, p.ThingID, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE photos SET position = 0 WHERE id = ?`, id)
		return err
	})
}

// DeletePhoto deletes a photo and returns it, so the caller can remove its file.
func (s *Store) DeletePhoto(ctx context.Context, hh, id int64) (*Photo, error) {
	p, err := s.GetPhoto(ctx, hh, id)
	if err != nil {
		return nil, err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM photos WHERE id = ?`, id); err != nil {
			return err
		}
		return touch(ctx, tx, p.ThingID)
	})
	return p, err
}
