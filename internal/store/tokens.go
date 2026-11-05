package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fhavrlent/wherehouse/internal/i18n"
)

// TokenPrefix starts every API token, so they are easy to recognise.
const TokenPrefix = "wherehouse_"

// MaxTokens is the most API tokens one person can have.
const MaxTokens = 20

type Token struct {
	ID int64 `json:"id"`
	// Household is the household the token works in.
	Household  int64   `json:"household"`
	Name       string  `json:"name"`
	CreatedAt  string  `json:"createdAt"`
	LastUsedAt *string `json:"lastUsedAt"`
}

// CreateToken makes an API token and returns it; only its hash is kept, so
// this is the only time it can be shown.
func (s *Store) CreateToken(ctx context.Context, userID, hh int64, name string) (string, *Token, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", nil, invalid("tokenName", nil)
	case utf8.RuneCountInString(name) > MaxTagName:
		return "", nil, invalid("nameLong", i18n.Args{"max": MaxTagName})
	}
	b := make([]byte, 24)
	rand.Read(b)
	raw := TokenPrefix + hex.EncodeToString(b)
	t := &Token{Household: hh, Name: name, CreatedAt: now()}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens WHERE user_id = ?`, userID).Scan(&count); err != nil {
			return err
		}
		if count >= MaxTokens {
			return invalid("tooManyTokens", i18n.Args{"max": MaxTokens})
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO api_tokens (user_id, household_id, name, token_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
			userID, hh, name, hashToken(raw), t.CreatedAt)
		if err != nil {
			return err
		}
		t.ID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return "", nil, err
	}
	return raw, t, nil
}

func (s *Store) ListTokens(ctx context.Context, userID int64) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, household_id, name, created_at, last_used_at FROM api_tokens WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Token{}
	for rows.Next() {
		var t Token
		var used sql.NullString
		if err := rows.Scan(&t.ID, &t.Household, &t.Name, &t.CreatedAt, &used); err != nil {
			return nil, err
		}
		if used.Valid {
			t.LastUsedAt = &used.String
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) DeleteToken(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	return affected(res)
}

// DeleteUserTokens ends all of a person's API tokens, e.g. when they lose
// access at the identity provider.
func (s *Store) DeleteUserTokens(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE user_id = ?`, userID)
	return err
}

// TokenAuth is which token a request was made with, and its household.
type TokenAuth struct {
	ID, Household int64
}

// TokenUser returns whose an API token is and the household it works in,
// noting that it was used. A token of someone no longer in that household
// doesn't work.
func (s *Store) TokenUser(ctx context.Context, raw string) (*User, TokenAuth, error) {
	if !strings.HasPrefix(raw, TokenPrefix) {
		return nil, TokenAuth{}, ErrNotFound
	}
	var u User
	var id, hh int64
	var used sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT t.id, t.household_id, t.last_used_at, u.id, u.email, u.name, u.picture_etag, u.is_admin
		FROM api_tokens t JOIN users u ON u.id = t.user_id
		JOIN household_members m ON m.household_id = t.household_id AND m.user_id = t.user_id
		WHERE t.token_hash = ?`, hashToken(raw),
	).Scan(&id, &hh, &used, &u.ID, &u.Email, &u.Name, &u.Picture, &u.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, TokenAuth{}, ErrNotFound
	}
	if err != nil {
		return nil, TokenAuth{}, err
	}
	u.Picture = pictureURL(u.Picture)
	// Once an hour is enough to tell whether a token is still in use.
	if t, err := time.Parse(timeLayout, used.String); !used.Valid || err != nil || time.Since(t) > time.Hour {
		s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, now(), id)
	}
	return &u, TokenAuth{ID: id, Household: hh}, nil
}

// ---- links ----

type Link struct {
	ID      int64  `json:"id"`
	ThingID int64  `json:"thingId"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	// Source is the app the link is from, "" for links people added; with
	// it come ExternalID and Data.
	Source     string          `json:"source"`
	ExternalID string          `json:"externalId,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	CreatedAt  string          `json:"createdAt"`
}

// MaxLinks is the most links one item can have.
const MaxLinks = 50

// AddLink links a thing to a web address. A link it already has is kept,
// with the new title.
func (s *Store) AddLink(ctx context.Context, hh, thing int64, rawURL, title string) (*Link, error) {
	rawURL, title = strings.TrimSpace(rawURL), strings.TrimSpace(title)
	if err := checkURL(rawURL); err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(title) > MaxName {
		return nil, invalid("nameLong", i18n.Args{"max": MaxName})
	}
	l := &Link{ThingID: thing, URL: rawURL, Title: title}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := owned(ctx, tx, hh, thing); err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `SELECT id, created_at FROM thing_links WHERE thing_id = ? AND url = ? AND source = ''`, thing, rawURL).
			Scan(&l.ID, &l.CreatedAt)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE thing_links SET title = ? WHERE id = ?`, title, l.ID)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM thing_links WHERE thing_id = ? AND source = ''`, thing).Scan(&count); err != nil {
			return err
		}
		if count >= MaxLinks {
			return invalid("tooManyLinks", i18n.Args{"max": MaxLinks})
		}
		l.CreatedAt = now()
		res, err := tx.ExecContext(ctx, `INSERT INTO thing_links (thing_id, url, title, created_at) VALUES (?, ?, ?, ?)`,
			thing, rawURL, title, l.CreatedAt)
		if err != nil {
			return err
		}
		l.ID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return nil, err
	}
	return l, nil
}

// checkURL makes sure a link is a web address.
func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(raw) > 2000 {
		return invalid("linkUrl", nil)
	}
	return nil
}

// DeleteLink removes a link. Removing an app's link also gives the purchase
// details it owned back to this app.
func (s *Store) DeleteLink(ctx context.Context, hh, id int64) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var thing int64
		var source string
		err := tx.QueryRowContext(ctx, `SELECT l.thing_id, l.source FROM thing_links l JOIN things t ON t.id = l.thing_id
			WHERE l.id = ? AND t.household_id = ?`, id, hh).Scan(&thing, &source)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM thing_links WHERE id = ?`, id); err != nil {
			return err
		}
		if source == "" {
			return nil
		}
		_, err = tx.ExecContext(ctx, `UPDATE things SET purchase_owner = '', updated_at = ? WHERE id = ? AND purchase_owner = ?`,
			now(), thing, source)
		return err
	})
}

func linksOf(ctx context.Context, q queryer, thing int64) ([]Link, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, thing_id, url, title, source, external_id, data, created_at
		FROM thing_links WHERE thing_id = ? ORDER BY source = '', id`, thing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		var l Link
		var data string
		if err := rows.Scan(&l.ID, &l.ThingID, &l.URL, &l.Title, &l.Source, &l.ExternalID, &data, &l.CreatedAt); err != nil {
			return nil, err
		}
		if data != "" {
			l.Data = json.RawMessage(data)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ---- hooks ----

// Hook is where to ping an app's token when its household changes.
type Hook struct {
	TokenID int64
	URL     string
	Secret  string
}

// SetTokenAppHost records the host a person approved for an app's token.
func (s *Store) SetTokenAppHost(ctx context.Context, tokenID int64, host string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET app_host = ? WHERE id = ?`, host, tokenID)
	return err
}

// TokenAppHost returns the host approved for a token, "" for tokens people
// made themselves.
func (s *Store) TokenAppHost(ctx context.Context, tokenID int64) (string, error) {
	var host string
	err := s.db.QueryRowContext(ctx, `SELECT app_host FROM api_tokens WHERE id = ?`, tokenID).Scan(&host)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return host, err
}

// SetHook saves (or with url "" removes) a token's hook and returns its
// new secret.
func (s *Store) SetHook(ctx context.Context, tokenID int64, url string) (string, error) {
	secret := ""
	if url != "" {
		b := make([]byte, 24)
		rand.Read(b)
		secret = hex.EncodeToString(b)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET hook_url = ?, hook_secret = ? WHERE id = ?`, url, secret, tokenID)
	return secret, err
}

// Hooks lists the hooks of a household's tokens, of people still in it.
func (s *Store) Hooks(ctx context.Context, hh int64) ([]Hook, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.hook_url, t.hook_secret FROM api_tokens t
		JOIN household_members m ON m.household_id = t.household_id AND m.user_id = t.user_id
		WHERE t.household_id = ? AND t.hook_url <> ''`, hh)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Hook
	for rows.Next() {
		var h Hook
		if err := rows.Scan(&h.TokenID, &h.URL, &h.Secret); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
