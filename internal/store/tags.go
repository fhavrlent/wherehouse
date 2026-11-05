package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/fhavrlent/wherehouse/internal/i18n"
)

type Tag struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	// Count is how many current things have the tag, in tag lists.
	Count int `json:"count,omitempty"`
}

// TagColors are the colours tags can have; new tags take them in turn.
var TagColors = []string{"slate", "red", "orange", "amber", "lime", "green", "teal", "sky", "blue", "violet", "pink"}

// MaxTagName is the longest tag name, in characters.
const MaxTagName = 40

func (s *Store) ListTags(ctx context.Context, hh int64) ([]Tag, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.color,
		       (SELECT COUNT(*) FROM thing_tags tt JOIN things t ON t.id = tt.thing_id
		        WHERE tt.tag_id = g.id AND t.archived_at IS NULL AND t.is_template = 0)
		FROM tags g WHERE g.household_id = ? ORDER BY g.name COLLATE NOCASE`, hh)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.Count); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTag(ctx context.Context, hh, id int64) (*Tag, error) {
	var t Tag
	err := s.db.QueryRowContext(ctx, `SELECT id, name, color FROM tags WHERE id = ? AND household_id = ?`, id, hh).Scan(&t.ID, &t.Name, &t.Color)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

func validColor(c string) bool {
	for _, x := range TagColors {
		if x == c {
			return true
		}
	}
	return false
}

// UpdateTag renames or recolours a tag. Renaming it to another tag's name
// merges the two.
func (s *Store) UpdateTag(ctx context.Context, hh, id int64, name, color string) error {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return invalid("tagName", nil)
	case utf8.RuneCountInString(name) > MaxTagName:
		return invalid("tagLong", i18n.Args{"max": MaxTagName})
	case !validColor(color):
		return invalid("tagColor", nil)
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var one int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM tags WHERE id = ? AND household_id = ?`, id, hh).Scan(&one); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		var other int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE household_id = ? AND name = ? AND id <> ?`, hh, name, id).Scan(&other)
		if err == nil {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO thing_tags (thing_id, tag_id) SELECT thing_id, ? FROM thing_tags WHERE tag_id = ?`, other, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, id); err != nil {
				return err
			}
			id = other
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE tags SET name = ?, color = ? WHERE id = ?`, name, color, id); err != nil {
			return err
		}
		return s.reindexTagged(ctx, tx, hh, id)
	})
}

func (s *Store) DeleteTag(ctx context.Context, hh, id int64) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		things, err := tagged(ctx, tx, id)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM tags WHERE id = ? AND household_id = ?`, id, hh)
		if err != nil {
			return err
		}
		if err := affected(res); err != nil {
			return err
		}
		return s.reindex(ctx, tx, hh, things...)
	})
}

func tagged(ctx context.Context, tx *sql.Tx, tag int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT thing_id FROM thing_tags WHERE tag_id = ?`, tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) reindexTagged(ctx context.Context, tx *sql.Tx, hh, tag int64) error {
	things, err := tagged(ctx, tx, tag)
	if err != nil {
		return err
	}
	return s.reindex(ctx, tx, hh, things...)
}

// normalizeTags trims names and drops empty and repeated ones.
func normalizeTags(names []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, n := range names {
		n = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(n), "#"))
		if n == "" || seen[strings.ToLower(n)] {
			continue
		}
		if utf8.RuneCountInString(n) > MaxTagName {
			return nil, invalid("tagLong", i18n.Args{"max": MaxTagName})
		}
		seen[strings.ToLower(n)] = true
		out = append(out, n)
	}
	return out, nil
}

// setThingTags gives a thing exactly these tags, adding tags that don't
// exist yet.
func setThingTags(ctx context.Context, tx *sql.Tx, hh, thing int64, names []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM thing_tags WHERE thing_id = ?`, thing); err != nil {
		return err
	}
	for _, n := range names {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE household_id = ? AND name = ?`, hh, n).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tags WHERE household_id = ?`, hh).Scan(&count); err != nil {
				return err
			}
			res, err := tx.ExecContext(ctx, `INSERT INTO tags (household_id, name, color) VALUES (?, ?, ?)`, hh, n, TagColors[count%len(TagColors)])
			if err != nil {
				return err
			}
			if id, err = res.LastInsertId(); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO thing_tags (thing_id, tag_id) VALUES (?, ?)`, thing, id); err != nil {
			return err
		}
	}
	return dropUnusedTags(ctx, tx, hh)
}

// dropUnusedTags deletes tags nothing has any more.
func dropUnusedTags(ctx context.Context, tx *sql.Tx, hh int64) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM tags WHERE household_id = ? AND id NOT IN (SELECT tag_id FROM thing_tags)`, hh)
	return err
}

// tagsOf returns the tags of each of the things, by thing.
func tagsOf(ctx context.Context, q queryer, things []int64) (map[int64][]Tag, error) {
	out := map[int64][]Tag{}
	if len(things) == 0 {
		return out, nil
	}
	args := make([]any, len(things))
	for i, id := range things {
		args[i] = id
	}
	rows, err := q.QueryContext(ctx, `
		SELECT tt.thing_id, g.id, g.name, g.color FROM thing_tags tt JOIN tags g ON g.id = tt.tag_id
		WHERE tt.thing_id IN (`+placeholders(len(things))+`) ORDER BY g.name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var thing int64
		var t Tag
		if err := rows.Scan(&thing, &t.ID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		out[thing] = append(out[thing], t)
	}
	return out, rows.Err()
}
