package store

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/fhavrlent/wherehouse/internal/i18n"
)

// KindRef is what a thing shows of its kind.
type KindRef struct {
	ID int64 `json:"id"`
	// Key names a built-in kind, whose name is in the reader's language
	// until it is renamed; "" for kinds people added.
	Key     string `json:"key"`
	Name    string `json:"name"`
	Icon    string `json:"icon"`
	IsPlace bool   `json:"isPlace"`
}

type Kind struct {
	KindRef
	Position int `json:"position"`
	// Count is how many things are of this kind.
	Count int `json:"count"`
}

// The kinds every household starts with. "thing" is what new things are
// unless something else is chosen, and can't be deleted.
var builtinKinds = []struct {
	key, icon string
	place     bool
}{
	{"building", "house", true},
	{"room", "door-open", true},
	{"furniture", "archive", true},
	{"box", "package", true},
	{"thing", "shapes", false},
}

func seedKinds(ctx context.Context, tx *sql.Tx, hh int64) error {
	for i, k := range builtinKinds {
		if _, err := tx.ExecContext(ctx, `INSERT INTO kinds (household_id, key, icon, is_place, position) VALUES (?, ?, ?, ?, ?)`,
			hh, k.key, k.icon, k.place, i); err != nil {
			return err
		}
	}
	return nil
}

// KindByKey returns a household's built-in kind.
func (s *Store) KindByKey(ctx context.Context, hh int64, key string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM kinds WHERE household_id = ? AND key = ?`, hh, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

func (s *Store) ListKinds(ctx context.Context, hh int64) ([]Kind, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT k.id, k.key, k.name, k.icon, k.is_place, k.position,
		       (SELECT COUNT(*) FROM things t WHERE t.kind_id = k.id)
		FROM kinds k WHERE k.household_id = ? ORDER BY k.is_place DESC, k.position, k.id`, hh)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Kind{}
	for rows.Next() {
		var k Kind
		var key sql.NullString
		if err := rows.Scan(&k.ID, &key, &k.Name, &k.Icon, &k.IsPlace, &k.Position, &k.Count); err != nil {
			return nil, err
		}
		k.Key = key.String
		out = append(out, k)
	}
	return out, rows.Err()
}

// iconRe is a built-in icon's name.
var iconRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// checkIcon makes sure icon is a built-in icon's name, or a custom icon of
// the household.
func checkIcon(ctx context.Context, q queryer, hh int64, icon string) error {
	if id, ok := strings.CutPrefix(icon, "custom:"); ok {
		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return invalid("icon", nil)
		}
		var one int
		if err := q.QueryRowContext(ctx, `SELECT 1 FROM icons WHERE id = ? AND household_id = ?`, n, hh).Scan(&one); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return invalid("icon", nil)
			}
			return err
		}
		return nil
	}
	if len(icon) > 40 || !iconRe.MatchString(icon) {
		return invalid("icon", nil)
	}
	return nil
}

type KindInput struct {
	Name     string `json:"name"`
	Icon     string `json:"icon"`
	IsPlace  bool   `json:"isPlace"`
	Position int    `json:"position"`
}

func (s *Store) CreateKind(ctx context.Context, hh int64, in KindInput) (*Kind, error) {
	in.Name = strings.TrimSpace(in.Name)
	switch {
	case in.Name == "":
		return nil, invalid("kindName", nil)
	case utf8.RuneCountInString(in.Name) > MaxTagName:
		return nil, invalid("nameLong", i18n.Args{"max": MaxTagName})
	}
	var id int64
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := checkIcon(ctx, tx, hh, in.Icon); err != nil {
			return err
		}
		var count, next int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(position) + 1, 0) FROM kinds WHERE household_id = ?`, hh).
			Scan(&count, &next); err != nil {
			return err
		}
		if count >= MaxKinds {
			return invalid("tooManyKinds", i18n.Args{"max": MaxKinds})
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO kinds (household_id, name, icon, is_place, position) VALUES (?, ?, ?, ?, ?)`,
			hh, in.Name, in.Icon, in.IsPlace, next)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.getKind(ctx, hh, id)
}

// MaxKinds is the most kinds a household can have.
const MaxKinds = 100

func (s *Store) getKind(ctx context.Context, hh, id int64) (*Kind, error) {
	kinds, err := s.ListKinds(ctx, hh)
	if err != nil {
		return nil, err
	}
	for _, k := range kinds {
		if k.ID == id {
			return &k, nil
		}
	}
	return nil, ErrNotFound
}

// UpdateKind renames a kind, changes its icon, whether it is a place, or
// its position. A built-in kind renamed to "" shows its own name again.
func (s *Store) UpdateKind(ctx context.Context, hh, id int64, in KindInput) (*Kind, error) {
	in.Name = strings.TrimSpace(in.Name)
	if utf8.RuneCountInString(in.Name) > MaxTagName {
		return nil, invalid("nameLong", i18n.Args{"max": MaxTagName})
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var key sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT key FROM kinds WHERE id = ? AND household_id = ?`, id, hh).Scan(&key); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if in.Name == "" && !key.Valid {
			return invalid("kindName", nil)
		}
		if err := checkIcon(ctx, tx, hh, in.Icon); err != nil {
			return err
		}
		// What new things are must stay a thing, not a place.
		if key.String == "thing" {
			in.IsPlace = false
		}
		if _, err := tx.ExecContext(ctx, `UPDATE kinds SET name = ?, icon = ?, is_place = ?, position = ? WHERE id = ?`,
			in.Name, in.Icon, in.IsPlace, in.Position, id); err != nil {
			return err
		}
		return s.reindexAll(ctx, tx, hh)
	})
	if err != nil {
		return nil, err
	}
	return s.getKind(ctx, hh, id)
}

// DeleteKind deletes a kind nothing is of.
func (s *Store) DeleteKind(ctx context.Context, hh, id int64) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var key sql.NullString
		var used int
		err := tx.QueryRowContext(ctx, `SELECT key, (SELECT COUNT(*) FROM things WHERE kind_id = kinds.id) FROM kinds
			WHERE id = ? AND household_id = ?`, id, hh).Scan(&key, &used)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		switch {
		case key.String == "thing":
			return invalid("kindNeeded", nil)
		case used > 0:
			return invalid("kindUsed", i18n.Args{"count": used})
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM kinds WHERE id = ?`, id)
		return err
	})
}

// ---- custom icons ----

type Icon struct {
	ID       int64  `json:"id"`
	File     string `json:"-"`
	MimeType string `json:"mimeType"`
}

// MaxIcons is the most custom icons a household can have.
const MaxIcons = 200

func (s *Store) AddIcon(ctx context.Context, hh int64, file, mimeType string) (*Icon, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM icons WHERE household_id = ?`, hh).Scan(&count); err != nil {
		return nil, err
	}
	if count >= MaxIcons {
		return nil, invalid("tooManyIcons", i18n.Args{"max": MaxIcons})
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO icons (household_id, file, mime_type, created_at) VALUES (?, ?, ?, ?)`,
		hh, file, mimeType, now())
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Icon{ID: id, File: file, MimeType: mimeType}, nil
}

func (s *Store) ListIcons(ctx context.Context, hh int64) ([]Icon, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, file, mime_type FROM icons WHERE household_id = ? ORDER BY id`, hh)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Icon{}
	for rows.Next() {
		var i Icon
		if err := rows.Scan(&i.ID, &i.File, &i.MimeType); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) GetIcon(ctx context.Context, hh, id int64) (*Icon, error) {
	var i Icon
	err := s.db.QueryRowContext(ctx, `SELECT id, file, mime_type FROM icons WHERE id = ? AND household_id = ?`, id, hh).
		Scan(&i.ID, &i.File, &i.MimeType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &i, err
}

// DeleteIcon deletes a custom icon no kind uses and returns it, so the
// caller can remove its file.
func (s *Store) DeleteIcon(ctx context.Context, hh, id int64) (*Icon, error) {
	i, err := s.GetIcon(ctx, hh, id)
	if err != nil {
		return nil, err
	}
	var used int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kinds WHERE household_id = ? AND icon = ?`,
		hh, "custom:"+strconv.FormatInt(id, 10)).Scan(&used); err != nil {
		return nil, err
	}
	if used > 0 {
		return nil, invalid("iconUsed", nil)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM icons WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return i, nil
}
