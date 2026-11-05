package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fhavrlent/wherehouse/internal/i18n"
)

// AppLink is a thing's link to what it is in another app, such as its
// warranty in Warranteed.
type AppLink struct {
	Source     string `json:"source"`
	ExternalID string `json:"externalId"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	// Data is what the app tells about it, shown here, e.g. when a warranty
	// ends. At most MaxAppData bytes of JSON.
	Data json.RawMessage `json:"data,omitempty"`
}

// MaxAppData is the most an app link's data can be, in bytes.
const MaxAppData = 4 << 10

var sourceRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,30}$`)

// AppLinkInput is what an app sends to link a thing. With Purchase, the app
// owns the thing's purchase and warranty details from then on; they are
// changed through it.
type AppLinkInput struct {
	ExternalID string          `json:"externalId"`
	URL        string          `json:"url"`
	Title      string          `json:"title"`
	Data       json.RawMessage `json:"data"`
	Purchase   *Purchase       `json:"purchase"`
}

func (in *AppLinkInput) normalize(source string) error {
	in.ExternalID, in.URL, in.Title = strings.TrimSpace(in.ExternalID), strings.TrimSpace(in.URL), strings.TrimSpace(in.Title)
	switch {
	case !sourceRe.MatchString(source):
		return invalid("appSource", nil)
	case in.ExternalID == "" || len(in.ExternalID) > 100:
		return invalid("appExternalId", nil)
	case utf8.RuneCountInString(in.Title) > MaxName:
		return invalid("nameLong", i18n.Args{"max": MaxName})
	case len(in.Data) > MaxAppData:
		return invalid("appDataLong", i18n.Args{"kb": MaxAppData >> 10})
	case len(in.Data) > 0 && !json.Valid(in.Data):
		return invalid("badRequest", nil)
	}
	if in.URL != "" {
		if err := checkURL(in.URL); err != nil {
			return err
		}
	}
	return nil
}

// SetAppLink links a thing to what it is in another app, replacing that
// app's earlier link. Linking it to something else of the app's moves the
// link.
func (s *Store) SetAppLink(ctx context.Context, hh, thing int64, source string, in AppLinkInput) error {
	if err := in.normalize(source); err != nil {
		return err
	}
	if in.Purchase != nil {
		p := ThingInput{Name: "x", Purchase: *in.Purchase}
		if err := p.normalize(); err != nil {
			return err
		}
		in.Purchase = &p.Purchase
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := owned(ctx, tx, hh, thing); err != nil {
			return err
		}
		// The same thing of the app can be linked to one thing here only.
		if _, err := tx.ExecContext(ctx, `DELETE FROM thing_links WHERE source = ? AND external_id = ? AND thing_id <> ?
			AND thing_id IN (SELECT id FROM things WHERE household_id = ?)`, source, in.ExternalID, thing, hh); err != nil {
			return err
		}
		// A plain link to the same page, e.g. copied from elsewhere, is
		// replaced by the app's.
		if _, err := tx.ExecContext(ctx, `DELETE FROM thing_links WHERE thing_id = ? AND source = '' AND url = ?`, thing, in.URL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO thing_links (thing_id, url, title, source, external_id, data, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (thing_id, source) WHERE source <> '' DO UPDATE SET
				url = excluded.url, title = excluded.title, external_id = excluded.external_id, data = excluded.data`,
			thing, in.URL, in.Title, source, in.ExternalID, string(in.Data), now()); err != nil {
			return err
		}
		if p := in.Purchase; p != nil {
			if _, err := tx.ExecContext(ctx, `
				UPDATE things SET purchase_owner = ?, purchase_date = ?, purchase_price = ?, purchase_from = ?, currency = ?,
					warranty_until = ?, lifetime_warranty = ?, warranty_details = ?, updated_at = ?
				WHERE id = ?`,
				source, p.PurchaseDate, p.PurchasePrice, p.PurchaseFrom, p.Currency, p.WarrantyUntil, p.LifetimeWarranty,
				p.WarrantyDetails, now(), thing); err != nil {
				return err
			}
		} else if err := touch(ctx, tx, thing); err != nil {
			return err
		}
		return s.reindex(ctx, tx, hh, thing)
	})
}

// RemoveAppLink unlinks a thing from an app. If the app owned its purchase
// details, they stay as they were and become this app's to change.
func (s *Store) RemoveAppLink(ctx context.Context, hh, thing int64, source string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := owned(ctx, tx, hh, thing); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM thing_links WHERE thing_id = ? AND source = ?`, thing, source)
		if err != nil {
			return err
		}
		if err := affected(res); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE things SET purchase_owner = '', updated_at = ? WHERE id = ? AND purchase_owner = ?`,
			now(), thing, source)
		return err
	})
}

// ThingByApp returns the thing linked to something of an app.
func (s *Store) ThingByApp(ctx context.Context, hh int64, source, externalID string) (*Thing, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT l.thing_id FROM thing_links l JOIN things t ON t.id = l.thing_id
		WHERE l.source = ? AND l.external_id = ? AND t.household_id = ?`, source, externalID, hh).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetThing(ctx, hh, id)
}

// Changes are the things changed after a time, and those deleted, for apps
// keeping in step.
type Changes struct {
	// Until is when to ask from next time.
	Until   string  `json:"until"`
	Things  []Thing `json:"things"`
	Deleted []int64 `json:"deleted"`
	// More: not everything came, ask again from Until.
	More bool `json:"more"`
}

// changesLimit is about how many things come at once.
const changesLimit = 1000

// ChangesSince lists the things of a household changed at or after since
// (RFC 3339), and those deleted then.
//
// When more changed than the limit, it stops at the second the last thing
// sent changed in, with all of that second, so asking again from Until
// misses nothing and never asks for the same again.
func (s *Store) ChangesSince(ctx context.Context, hh int64, since time.Time) (*Changes, error) {
	until := time.Now().UTC().Truncate(time.Second)
	from := since.UTC().Format(timeLayout)
	t, err := s.loadTree(ctx, s.db, hh)
	if err != nil {
		return nil, err
	}
	things, err := s.queryThings(ctx, t, `SELECT `+thingColumns+thingFrom+`
		WHERE t.household_id = ? AND t.updated_at >= ? AND t.is_template = 0 ORDER BY t.updated_at, t.id LIMIT ?`,
		hh, from, changesLimit)
	if err != nil {
		return nil, err
	}
	out := &Changes{Things: things, Deleted: []int64{}}
	to := until.Format(timeLayout)
	if len(things) == changesLimit {
		last := things[len(things)-1]
		rest, err := s.queryThings(ctx, t, `SELECT `+thingColumns+thingFrom+`
			WHERE t.household_id = ? AND t.updated_at = ? AND t.id > ? AND t.is_template = 0 ORDER BY t.id`,
			hh, last.UpdatedAt, last.ID)
		if err != nil {
			return nil, err
		}
		out.Things = append(out.Things, rest...)
		at, err := time.Parse(timeLayout, last.UpdatedAt)
		if err != nil {
			return nil, err
		}
		// A second that is over can't change any more; the current one can,
		// so it is asked for again.
		to, out.More = last.UpdatedAt, true
		if at.Before(until) {
			until = at.Add(time.Second)
		} else {
			until = at
		}
	}
	out.Until = until.Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, `SELECT thing_id FROM deleted_things
		WHERE household_id = ? AND deleted_at >= ? AND deleted_at <= ?`, hh, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out.Deleted = append(out.Deleted, id)
	}
	return out, rows.Err()
}
