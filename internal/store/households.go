package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fhavrlent/wherehouse/internal/i18n"
)

// Roles in a household.
const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

type Household struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
	// Role is the reader's role in it.
	Role    string `json:"role"`
	Members int    `json:"members"`
}

type Member struct {
	UserID   int64  `json:"userId"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	JoinedAt string `json:"joinedAt"`
}

// InviteDays is how long an invite link works.
const InviteDays = 7

func createHousehold(ctx context.Context, tx *sql.Tx, uid int64, name string) (int64, error) {
	res, err := tx.ExecContext(ctx, `INSERT INTO households (name, created_at) VALUES (?, ?)`, name, now())
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO household_members (household_id, user_id, role, joined_at) VALUES (?, ?, 'owner', ?)`,
		id, uid, now()); err != nil {
		return 0, err
	}
	if err := seedKinds(ctx, tx, id); err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE user_settings SET current_household = ? WHERE user_id = ?`, id, uid)
	return id, err
}

// CurrentHousehold returns the household the person works in: the one they
// chose, else the first they are in. Someone in none gets their own, named
// name.
func (s *Store) CurrentHousehold(ctx context.Context, uid int64, name string) (int64, error) {
	var hh int64
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			SELECT m.household_id FROM household_members m
			LEFT JOIN user_settings s ON s.user_id = m.user_id
			WHERE m.user_id = ?
			ORDER BY m.household_id = s.current_household DESC, m.joined_at, m.household_id LIMIT 1`, uid).Scan(&hh)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE user_settings SET current_household = ? WHERE user_id = ? AND
				(current_household IS NULL OR current_household <> ?)`, hh, uid, hh)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		hh, err = createHousehold(ctx, tx, uid, name)
		return err
	})
	return hh, err
}

// Role returns a person's role in a household, or ErrNotFound when they
// aren't in it.
func (s *Store) Role(ctx context.Context, hh, uid int64) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx, `SELECT role FROM household_members WHERE household_id = ? AND user_id = ?`, hh, uid).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return role, err
}

func (s *Store) ListHouseholds(ctx context.Context, uid int64) ([]Household, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.id, h.name, h.currency, m.role, (SELECT COUNT(*) FROM household_members x WHERE x.household_id = h.id)
		FROM households h JOIN household_members m ON m.household_id = h.id
		WHERE m.user_id = ? ORDER BY h.name COLLATE NOCASE, h.id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Household{}
	for rows.Next() {
		var h Household
		if err := rows.Scan(&h.ID, &h.Name, &h.Currency, &h.Role, &h.Members); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) GetHousehold(ctx context.Context, hh int64) (*Household, error) {
	var h Household
	err := s.db.QueryRowContext(ctx, `SELECT id, name, currency, (SELECT COUNT(*) FROM household_members WHERE household_id = households.id)
		FROM households WHERE id = ?`, hh).Scan(&h.ID, &h.Name, &h.Currency, &h.Members)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &h, err
}

func normalizeHousehold(name, currency string) (string, string, error) {
	name, currency = strings.TrimSpace(name), strings.ToUpper(strings.TrimSpace(currency))
	switch {
	case name == "":
		return "", "", invalid("householdName", nil)
	case utf8.RuneCountInString(name) > MaxName:
		return "", "", invalid("nameLong", i18n.Args{"max": MaxName})
	case !currencyRe(currency):
		return "", "", invalid("currency", nil)
	}
	return name, currency, nil
}

// CreateHousehold adds a household with uid as its owner and switches them
// to it.
func (s *Store) CreateHousehold(ctx context.Context, uid int64, name, currency string) (int64, error) {
	name, currency, err := normalizeHousehold(name, currency)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM household_members WHERE user_id = ?`, uid).Scan(&count); err != nil {
			return err
		}
		if count >= MaxHouseholds {
			return invalid("tooManyHouseholds", i18n.Args{"max": MaxHouseholds})
		}
		if id, err = createHousehold(ctx, tx, uid, name); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE households SET currency = ? WHERE id = ?`, currency, id)
		return err
	})
	return id, err
}

// MaxHouseholds is the most households one person can be in.
const MaxHouseholds = 20

func (s *Store) UpdateHousehold(ctx context.Context, hh int64, name, currency string) error {
	name, currency, err := normalizeHousehold(name, currency)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE households SET name = ?, currency = ? WHERE id = ?`, name, currency, hh)
	if err != nil {
		return err
	}
	return affected(res)
}

// SwitchHousehold makes hh the household uid works in.
func (s *Store) SwitchHousehold(ctx context.Context, uid, hh int64) error {
	if _, err := s.Role(ctx, hh, uid); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE user_settings SET current_household = ? WHERE user_id = ?`, hh, uid)
	return err
}

func (s *Store) Members(ctx context.Context, hh int64) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.name, u.email, m.role, m.joined_at
		FROM household_members m JOIN users u ON u.id = m.user_id
		WHERE m.household_id = ? ORDER BY m.role = 'owner' DESC, u.name COLLATE NOCASE`, hh)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Name, &m.Email, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetRole makes a member an owner or a member. The last owner stays one.
func (s *Store) SetRole(ctx context.Context, hh, uid int64, role string) error {
	if role != RoleOwner && role != RoleMember {
		return invalid("badRequest", nil)
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if role == RoleMember {
			if err := keepAnOwner(ctx, tx, hh, uid); err != nil {
				return err
			}
		}
		res, err := tx.ExecContext(ctx, `UPDATE household_members SET role = ? WHERE household_id = ? AND user_id = ?`, role, hh, uid)
		if err != nil {
			return err
		}
		return affected(res)
	})
}

// keepAnOwner refuses to leave a household with members but no owner when
// uid stops being one.
func keepAnOwner(ctx context.Context, tx *sql.Tx, hh, uid int64) error {
	var owners, members int
	var isOwner bool
	if err := tx.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM household_members WHERE household_id = ? AND role = 'owner'),
		(SELECT COUNT(*) FROM household_members WHERE household_id = ?),
		EXISTS (SELECT 1 FROM household_members WHERE household_id = ? AND user_id = ? AND role = 'owner')`,
		hh, hh, hh, uid).Scan(&owners, &members, &isOwner); err != nil {
		return err
	}
	if isOwner && owners == 1 && members > 1 {
		return invalid("lastOwner", nil)
	}
	return nil
}

// RemoveMember takes someone out of a household, or lets them leave. The
// last one to leave deletes the household; then the files of everything in
// it are returned for the caller to remove.
func (s *Store) RemoveMember(ctx context.Context, hh, uid int64) ([]string, error) {
	var files []string
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := keepAnOwner(ctx, tx, hh, uid); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM household_members WHERE household_id = ? AND user_id = ?`, hh, uid)
		if err != nil {
			return err
		}
		if err := affected(res); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM api_tokens WHERE household_id = ? AND user_id = ?`, hh, uid); err != nil {
			return err
		}
		var left int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM household_members WHERE household_id = ?`, hh).Scan(&left); err != nil {
			return err
		}
		if left == 0 {
			files, err = deleteHousehold(ctx, tx, hh)
		}
		return err
	})
	return files, err
}

func deleteHousehold(ctx context.Context, tx *sql.Tx, hh int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT p.file FROM photos p JOIN things t ON t.id = p.thing_id WHERE t.household_id = ?
		UNION ALL SELECT a.file FROM attachments a JOIN things t ON t.id = a.thing_id WHERE t.household_id = ? AND a.file <> ''
		UNION ALL SELECT file FROM icons WHERE household_id = ?`, hh, hh, hh)
	if err != nil {
		return nil, err
	}
	var files []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			rows.Close()
			return nil, err
		}
		files = append(files, f)
	}
	rows.Close()
	// Things hold each other, so they go before the household's cascade.
	for _, q := range []string{
		`UPDATE things SET parent_id = NULL WHERE household_id = ?`,
		`DELETE FROM things WHERE household_id = ?`,
		`DELETE FROM search WHERE household = ?`,
		`DELETE FROM households WHERE id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, q, hh); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// CreateInvite makes an invite link's secret for a household.
func (s *Store) CreateInvite(ctx context.Context, hh, uid int64) (string, time.Time, error) {
	b := make([]byte, 18)
	rand.Read(b)
	raw := hex.EncodeToString(b)
	expires := time.Now().Add(InviteDays * 24 * time.Hour).UTC()
	_, err := s.db.ExecContext(ctx, `INSERT INTO invites (token_hash, household_id, created_by, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		hashToken(raw), hh, uid, expires.Format(timeLayout), now())
	return raw, expires, err
}

// Invite returns the household an invite link is for.
func (s *Store) Invite(ctx context.Context, raw string) (*Household, error) {
	var hh int64
	err := s.db.QueryRowContext(ctx, `SELECT household_id FROM invites WHERE token_hash = ? AND expires_at > ?`, hashToken(raw), now()).Scan(&hh)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetHousehold(ctx, hh)
}

// AcceptInvite adds uid to the invite's household and switches them to it.
// A household of their own that is still empty, as made for them at
// sign-in, goes away. The link keeps working for others until it expires.
func (s *Store) AcceptInvite(ctx context.Context, raw string, uid int64) (int64, error) {
	h, err := s.Invite(ctx, raw)
	if err != nil {
		return 0, err
	}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO household_members (household_id, user_id, role, joined_at) VALUES (?, ?, 'member', ?)
			ON CONFLICT DO NOTHING`, h.ID, uid, now()); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT m.household_id FROM household_members m WHERE m.user_id = ? AND m.household_id <> ?
			AND (SELECT COUNT(*) FROM household_members x WHERE x.household_id = m.household_id) = 1
			AND NOT EXISTS (SELECT 1 FROM things t WHERE t.household_id = m.household_id)`, uid, h.ID)
		if err != nil {
			return err
		}
		var empty []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			empty = append(empty, id)
		}
		rows.Close()
		for _, id := range empty {
			if _, err := deleteHousehold(ctx, tx, id); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE user_settings SET current_household = ? WHERE user_id = ?`, h.ID, uid)
		return err
	})
	return h.ID, err
}

// HouseholdsOf lists the households a person is in, for reminders.
func (s *Store) HouseholdsOf(ctx context.Context, uid int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT household_id FROM household_members WHERE user_id = ?`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
