package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"regexp"
	"strings"

	"github.com/fhavrlent/wherehouse/internal/i18n"
)

// codeAlphabet leaves out 0, 1, I and O, which are easily mistaken for
// each other when a code is typed in.
const codeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// CodeLength gives 32^5, about 33 million, codes.
const CodeLength = 5

// MaxBlankLabels is the most blank labels made at once.
const MaxBlankLabels = 500

func newCode() string {
	b := make([]byte, CodeLength)
	rand.Read(b)
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b)
}

// codeRe is what a label code may look like: made here, or kept from
// HomeBox asset IDs such as "000-123".
var codeRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{0,19}$`)

// NormalizeCode upper-cases a typed or scanned code and reports whether it
// can be one.
func NormalizeCode(code string) (string, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	return code, codeRe.MatchString(code)
}

type Label struct {
	Code    string `json:"code"`
	ThingID *int64 `json:"thingId"`
}

// GetLabel returns a label of household hh.
func (s *Store) GetLabel(ctx context.Context, hh int64, code string) (*Label, error) {
	code, ok := NormalizeCode(code)
	if !ok {
		return nil, ErrNotFound
	}
	var l Label
	var thing sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT code, thing_id FROM labels WHERE code = ? AND household_id = ?`, code, hh).Scan(&l.Code, &thing)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if thing.Valid {
		l.ThingID = &thing.Int64
	}
	return &l, nil
}

// LabelHousehold returns which household a label is of, if any, so a scan
// in another household can switch to it.
func (s *Store) LabelHousehold(ctx context.Context, code string) (int64, error) {
	code, ok := NormalizeCode(code)
	if !ok {
		return 0, ErrNotFound
	}
	var hh int64
	err := s.db.QueryRowContext(ctx, `SELECT household_id FROM labels WHERE code = ?`, code).Scan(&hh)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return hh, err
}

// BlankLabels makes count new labels that aren't on anything yet.
func (s *Store) BlankLabels(ctx context.Context, hh int64, count int) ([]string, error) {
	if count < 1 || count > MaxBlankLabels {
		return nil, invalid("labelCount", i18n.Args{"max": MaxBlankLabels})
	}
	var codes []string
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		for len(codes) < count {
			code := newCode()
			res, err := tx.ExecContext(ctx, `INSERT INTO labels (code, household_id, created_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
				code, hh, now())
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 1 {
				codes = append(codes, code)
			}
		}
		return nil
	})
	return codes, err
}

// AttachLabel puts a label on a thing, replacing any label it had. With
// code empty, the thing keeps the label it has or gets a new one. A code
// that doesn't exist yet is added, so labels printed elsewhere (or
// HomeBox's) can be used.
func (s *Store) AttachLabel(ctx context.Context, hh, thing int64, code string) (string, error) {
	if code != "" {
		var ok bool
		if code, ok = NormalizeCode(code); !ok {
			return "", invalid("labelCode", nil)
		}
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := owned(ctx, tx, hh, thing); err != nil {
			return err
		}
		var current string
		err := tx.QueryRowContext(ctx, `SELECT code FROM labels WHERE thing_id = ?`, thing).Scan(&current)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if code == "" {
			if current != "" {
				code = current
				return nil
			}
			for {
				code = newCode()
				res, err := tx.ExecContext(ctx, `INSERT INTO labels (code, household_id, thing_id, created_at) VALUES (?, ?, ?, ?)
					ON CONFLICT DO NOTHING`, code, hh, thing, now())
				if err != nil {
					return err
				}
				if n, _ := res.RowsAffected(); n == 1 {
					return s.reindex(ctx, tx, hh, thing)
				}
			}
		}
		if code == current {
			return nil
		}
		var on sql.NullInt64
		var of int64
		err = tx.QueryRowContext(ctx, `SELECT thing_id, household_id FROM labels WHERE code = ?`, code).Scan(&on, &of)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.ExecContext(ctx, `INSERT INTO labels (code, household_id, created_at) VALUES (?, ?, ?)`, code, hh, now()); err != nil {
				return err
			}
		case err != nil:
			return err
		case on.Valid || of != hh:
			return invalid("labelUsed", nil)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE labels SET thing_id = NULL WHERE thing_id = ?`, thing); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE labels SET thing_id = ? WHERE code = ?`, thing, code); err != nil {
			return err
		}
		return s.reindex(ctx, tx, hh, thing)
	})
	return code, err
}

// DetachLabel takes a label off what it is on; it can be used again.
func (s *Store) DetachLabel(ctx context.Context, hh int64, code string) error {
	l, err := s.GetLabel(ctx, hh, code)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE labels SET thing_id = NULL WHERE code = ?`, l.Code); err != nil {
			return err
		}
		if l.ThingID == nil {
			return nil
		}
		return s.reindex(ctx, tx, hh, *l.ThingID)
	})
}
