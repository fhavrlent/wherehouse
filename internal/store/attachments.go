package store

import (
	"context"
	"database/sql"
	"errors"
)

// Where an attachment is kept.
const (
	StorageLocal     = "local"
	StoragePaperless = "paperless"
)

// ValidKind reports whether k is a kind of document.
func ValidKind(k string) bool {
	switch k {
	case "manual", "receipt", "warranty", "other":
		return true
	}
	return false
}

type Attachment struct {
	ID             int64  `json:"id"`
	ThingID        int64  `json:"thingId"`
	Kind           string `json:"kind"`
	Storage        string `json:"storage"`
	Filename       string `json:"filename"`
	MimeType       string `json:"mimeType"`
	Size           int64  `json:"size"`
	File           string `json:"-"`
	PaperlessDocID *int64 `json:"paperlessDocId"`
	// AddedBy is whose Paperless connection opens a linked document.
	AddedBy   *int64 `json:"-"`
	CreatedAt string `json:"createdAt"`
}

const attachmentColumns = `a.id, a.thing_id, a.kind, a.storage, a.filename, a.mime_type, a.size, a.file, a.paperless_doc_id, a.added_by, a.created_at`

func scanAttachment(sc interface{ Scan(...any) error }) (*Attachment, error) {
	var a Attachment
	var doc, by sql.NullInt64
	if err := sc.Scan(&a.ID, &a.ThingID, &a.Kind, &a.Storage, &a.Filename, &a.MimeType, &a.Size, &a.File, &doc, &by, &a.CreatedAt); err != nil {
		return nil, err
	}
	if doc.Valid {
		a.PaperlessDocID = &doc.Int64
	}
	if by.Valid {
		a.AddedBy = &by.Int64
	}
	return &a, nil
}

func attachmentsOf(ctx context.Context, q queryer, thing int64) ([]Attachment, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+attachmentColumns+` FROM attachments a WHERE a.thing_id = ? ORDER BY a.created_at, a.id`, thing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Attachment{}
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (s *Store) AddAttachment(ctx context.Context, hh int64, a *Attachment) error {
	if !ValidKind(a.Kind) {
		return invalid("attachmentKind", nil)
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := owned(ctx, tx, hh, a.ThingID); err != nil {
			return err
		}
		if a.PaperlessDocID != nil {
			var dup int
			if tx.QueryRowContext(ctx, `SELECT 1 FROM attachments WHERE thing_id = ? AND paperless_doc_id = ?`,
				a.ThingID, *a.PaperlessDocID).Scan(&dup) == nil {
				return invalid("alreadyLinked", nil)
			}
		}
		a.CreatedAt = now()
		res, err := tx.ExecContext(ctx, `
			INSERT INTO attachments (thing_id, kind, storage, filename, mime_type, size, file, paperless_doc_id, added_by, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			a.ThingID, a.Kind, a.Storage, a.Filename, a.MimeType, a.Size, a.File, a.PaperlessDocID, a.AddedBy, a.CreatedAt)
		if err != nil {
			return err
		}
		if a.ID, err = res.LastInsertId(); err != nil {
			return err
		}
		return touch(ctx, tx, a.ThingID)
	})
}

func (s *Store) GetAttachment(ctx context.Context, hh, id int64) (*Attachment, error) {
	a, err := scanAttachment(s.db.QueryRowContext(ctx, `SELECT `+attachmentColumns+` FROM attachments a
		JOIN things t ON t.id = a.thing_id WHERE a.id = ? AND t.household_id = ?`, id, hh))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// UpdateAttachment changes a document's kind and name.
func (s *Store) UpdateAttachment(ctx context.Context, hh, id int64, kind, filename string) (*Attachment, error) {
	if !ValidKind(kind) {
		return nil, invalid("attachmentKind", nil)
	}
	if filename == "" {
		return nil, invalid("attachmentName", nil)
	}
	a, err := s.GetAttachment(ctx, hh, id)
	if err != nil {
		return nil, err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE attachments SET kind = ?, filename = ? WHERE id = ?`, kind, filename, id); err != nil {
			return err
		}
		return touch(ctx, tx, a.ThingID)
	})
	if err != nil {
		return nil, err
	}
	return s.GetAttachment(ctx, hh, id)
}

// DeleteAttachment deletes a document and returns it, so the caller can
// remove its file.
func (s *Store) DeleteAttachment(ctx context.Context, hh, id int64) (*Attachment, error) {
	a, err := s.GetAttachment(ctx, hh, id)
	if err != nil {
		return nil, err
	}
	// Its thing changed: apps following the change feed learn it's gone.
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM attachments WHERE id = ?`, id); err != nil {
			return err
		}
		return touch(ctx, tx, a.ThingID)
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ThingsWithDocument lists the household's things a Paperless document is
// linked to.
func (s *Store) ThingsWithDocument(ctx context.Context, hh, doc int64) ([]Thing, error) {
	t, err := s.loadTree(ctx, s.db, hh)
	if err != nil {
		return nil, err
	}
	return s.queryThings(ctx, t, `SELECT `+thingColumns+thingFrom+` WHERE t.household_id = ? AND t.id IN
		(SELECT thing_id FROM attachments WHERE paperless_doc_id = ?) ORDER BY t.name COLLATE NOCASE`, hh, doc)
}
