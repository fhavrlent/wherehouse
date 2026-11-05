package store

import (
	"context"
	"database/sql"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fhavrlent/wherehouse/internal/i18n"
)

// Field types.
const (
	FieldText   = "text"
	FieldNumber = "number"
	FieldDate   = "date"
	FieldURL    = "url"
	FieldBool   = "bool"
)

// MaxFields is the most custom fields there can be.
const MaxFields = 50

// Field is a custom field every thing can fill in.
type Field struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Position int    `json:"position"`
}

// FieldValue is a thing's value for a custom field.
type FieldValue struct {
	FieldID int64  `json:"fieldId"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
}

func validFieldType(t string) bool {
	switch t {
	case FieldText, FieldNumber, FieldDate, FieldURL, FieldBool:
		return true
	}
	return false
}

func (s *Store) ListFields(ctx context.Context, hh int64) ([]Field, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, type, position FROM fields WHERE household_id = ? ORDER BY position, id`, hh)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Field{}
	for rows.Next() {
		var f Field
		if err := rows.Scan(&f.ID, &f.Name, &f.Type, &f.Position); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func normalizeField(name, typ string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", invalid("fieldName", nil)
	case utf8.RuneCountInString(name) > MaxTagName:
		return "", invalid("fieldLong", i18n.Args{"max": MaxTagName})
	case !validFieldType(typ):
		return "", invalid("fieldType", nil)
	}
	return name, nil
}

func (s *Store) CreateField(ctx context.Context, hh int64, name, typ string) (*Field, error) {
	name, err := normalizeField(name, typ)
	if err != nil {
		return nil, err
	}
	f := &Field{Name: name, Type: typ}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(position) + 1, 0) FROM fields WHERE household_id = ?`, hh).
			Scan(&count, &f.Position); err != nil {
			return err
		}
		if count >= MaxFields {
			return invalid("tooManyFields", i18n.Args{"max": MaxFields})
		}
		var dup int
		if tx.QueryRowContext(ctx, `SELECT 1 FROM fields WHERE household_id = ? AND name = ?`, hh, name).Scan(&dup) == nil {
			return invalid("fieldExists", nil)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO fields (household_id, name, type, position) VALUES (?, ?, ?, ?)`, hh, name, typ, f.Position)
		if err != nil {
			return err
		}
		f.ID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

// UpdateField renames a field or moves it to position. Its type can't
// change, since values were checked against it.
func (s *Store) UpdateField(ctx context.Context, hh, id int64, name string, position int) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var typ string
		if err := tx.QueryRowContext(ctx, `SELECT type FROM fields WHERE id = ? AND household_id = ?`, id, hh).Scan(&typ); err != nil {
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		name, err := normalizeField(name, typ)
		if err != nil {
			return err
		}
		var dup int
		if tx.QueryRowContext(ctx, `SELECT 1 FROM fields WHERE household_id = ? AND name = ? AND id <> ?`, hh, name, id).Scan(&dup) == nil {
			return invalid("fieldExists", nil)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE fields SET name = ?, position = ? WHERE id = ?`, name, position, id); err != nil {
			return err
		}
		return s.reindexAll(ctx, tx, hh)
	})
}

func (s *Store) DeleteField(ctx context.Context, hh, id int64) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM fields WHERE id = ? AND household_id = ?`, id, hh)
		if err != nil {
			return err
		}
		if err := affected(res); err != nil {
			return err
		}
		return s.reindexAll(ctx, tx, hh)
	})
}

// normalizeValue checks a value against its field's type and returns it as
// stored; "" clears it.
func normalizeValue(f Field, v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	bad := func() (string, error) { return "", invalid("fieldValue", i18n.Args{"name": f.Name}) }
	switch f.Type {
	case FieldNumber:
		// "1 299,50" as well as "1299.50".
		plain := strings.NewReplacer(" ", "", "\u00a0", "", "\u202f", "", ",", ".").Replace(v)
		n, err := strconv.ParseFloat(plain, 64)
		if err != nil {
			return bad()
		}
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	case FieldDate:
		if _, err := time.Parse(DateLayout, v); err != nil {
			return bad()
		}
	case FieldURL:
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return bad()
		}
	case FieldBool:
		if v != "true" && v != "false" {
			return bad()
		}
		if v == "false" {
			return "", nil
		}
	}
	if utf8.RuneCountInString(v) > MaxName*5 {
		return "", invalid("textLong", i18n.Args{"max": MaxName * 5})
	}
	return v, nil
}

// setThingFields saves a thing's values, by field ID; fields left out keep
// their values.
func setThingFields(ctx context.Context, tx *sql.Tx, hh, thing int64, values map[int64]string) error {
	if len(values) == 0 {
		return nil
	}
	fields := map[int64]Field{}
	rows, err := tx.QueryContext(ctx, `SELECT id, name, type FROM fields WHERE household_id = ?`, hh)
	if err != nil {
		return err
	}
	for rows.Next() {
		var f Field
		if err := rows.Scan(&f.ID, &f.Name, &f.Type); err != nil {
			rows.Close()
			return err
		}
		fields[f.ID] = f
	}
	rows.Close()
	for id, raw := range values {
		f, ok := fields[id]
		if !ok {
			return invalid("fieldMissing", nil)
		}
		v, err := normalizeValue(f, raw)
		if err != nil {
			return err
		}
		if v == "" {
			_, err = tx.ExecContext(ctx, `DELETE FROM thing_fields WHERE thing_id = ? AND field_id = ?`, thing, id)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO thing_fields (thing_id, field_id, value) VALUES (?, ?, ?)
				ON CONFLICT (thing_id, field_id) DO UPDATE SET value = excluded.value`, thing, id, v)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func fieldsOf(ctx context.Context, q queryer, thing int64) ([]FieldValue, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT f.id, f.name, f.type, v.value FROM thing_fields v JOIN fields f ON f.id = v.field_id
		WHERE v.thing_id = ? ORDER BY f.position, f.id`, thing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FieldValue{}
	for rows.Next() {
		var v FieldValue
		if err := rows.Scan(&v.FieldID, &v.Name, &v.Type, &v.Value); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
