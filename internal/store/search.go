package store

import (
	"context"
	"database/sql"
	"strings"
	"unicode"
)

// Search rows are kept in step with things by the store's own changes.

// reindex rewrites the search rows of things.
func (s *Store) reindex(ctx context.Context, tx *sql.Tx, hh int64, ids ...int64) error {
	t, err := s.loadTree(ctx, tx, hh)
	if err != nil {
		return err
	}
	return s.indexThings(ctx, tx, hh, t, ids...)
}

func (s *Store) indexThings(ctx context.Context, tx *sql.Tx, hh int64, t tree, ids ...int64) error {
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM search WHERE ref = ?`, id); err != nil {
			return err
		}
		var parent sql.NullInt64
		var name, details string
		err := tx.QueryRowContext(ctx, `
			SELECT t.parent_id, t.name, concat_ws(' ', t.description, t.manufacturer, t.model, t.serial, t.notes,
				t.purchase_from, t.warranty_details, t.sold_to, t.sold_notes,
				(SELECT code FROM labels WHERE thing_id = t.id),
				(SELECT group_concat(g.name, ' ') FROM thing_tags tt JOIN tags g ON g.id = tt.tag_id WHERE tt.thing_id = t.id),
				(SELECT group_concat(value, ' ') FROM thing_fields WHERE thing_id = t.id AND value <> 'true'),
				CASE WHEN k.key IS NULL THEN k.name ELSE '' END)
			FROM things t JOIN kinds k ON k.id = t.kind_id WHERE t.id = ?`, id).Scan(&parent, &name, &details)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		var p *int64
		if parent.Valid {
			p = &parent.Int64
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO search (household, ref, name, details, path) VALUES (?, ?, ?, ?, ?)`,
			hh, id, name, details, t.pathText(p)); err != nil {
			return err
		}
	}
	return nil
}

// reindexInside rewrites the search rows of a thing and everything in it,
// as their paths change with it.
func (s *Store) reindexInside(ctx context.Context, tx *sql.Tx, hh, id int64) error {
	t, err := s.loadTree(ctx, tx, hh)
	if err != nil {
		return err
	}
	return s.indexThings(ctx, tx, hh, t, t.inside(id)...)
}

func (s *Store) reindexAll(ctx context.Context, tx *sql.Tx, hh int64) error {
	t, err := s.loadTree(ctx, tx, hh)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM search WHERE household = ?`, hh); err != nil {
		return err
	}
	ids := make([]int64, 0, len(t))
	for id := range t {
		ids = append(ids, id)
	}
	return s.indexThings(ctx, tx, hh, t, ids...)
}

// RebuildSearch rewrites the whole search index.
func (s *Store) RebuildSearch(ctx context.Context) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM search`); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT id FROM households`)
		if err != nil {
			return err
		}
		var hhs []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			hhs = append(hhs, id)
		}
		rows.Close()
		for _, hh := range hhs {
			if err := s.reindexAll(ctx, tx, hh); err != nil {
				return err
			}
		}
		return nil
	})
}

// matchQuery turns what someone typed into an FTS5 query: every word must
// match the start of a word, in any column.
func matchQuery(q string) string {
	words := strings.FieldsFunc(q, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	for i, w := range words {
		words[i] = `"` + w + `"*`
	}
	return strings.Join(words, " AND ")
}

// SearchLimit is the most things a search returns.
const SearchLimit = 100

// Search finds things whose name, details or the things they are in
// contain every word of q. Names count most.
func (s *Store) Search(ctx context.Context, hh int64, q string, archived bool) ([]Thing, error) {
	match := matchQuery(q)
	if match == "" {
		return []Thing{}, nil
	}
	t, err := s.loadTree(ctx, s.db, hh)
	if err != nil {
		return nil, err
	}
	cond := "t.archived_at IS NULL"
	if archived {
		cond = "t.archived_at IS NOT NULL"
	}
	return s.queryThings(ctx, t, `
		SELECT `+thingColumns+` FROM search s JOIN things t ON t.id = s.ref JOIN kinds k ON k.id = t.kind_id
		WHERE search MATCH ? AND s.household = ? AND t.is_template = 0 AND `+cond+`
		ORDER BY bm25(search, 0, 0, 10, 2, 1) LIMIT ?`, match, hh, SearchLimit)
}
