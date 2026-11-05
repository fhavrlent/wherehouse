package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/db"
	"github.com/fhavrlent/wherehouse/internal/secret"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	key, err := secret.LoadKey("", dir)
	if err != nil {
		t.Fatal(err)
	}
	return New(database, key), dir
}

func TestPaperlessTokenIsEncrypted(t *testing.T) {
	st, _ := newTestStore(t)
	ctx := context.Background()
	u, err := st.UpsertUser(ctx, UpsertUser{Issuer: "test", Subject: "1"})
	if err != nil {
		t.Fatal(err)
	}
	stored := func() string {
		var v string
		if err := st.db.QueryRow(`SELECT paperless_token FROM user_settings WHERE user_id = ?`, u.ID).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	token := func() string {
		s, err := st.Settings(ctx, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		return s.PaperlessToken
	}

	s, _ := st.Settings(ctx, u.ID)
	s.PaperlessToken = "secret-token"
	if err := st.SaveSettings(ctx, u.ID, s); err != nil {
		t.Fatal(err)
	}
	if v := stored(); !strings.HasPrefix(v, sealedPrefix) || strings.Contains(v, "secret-token") {
		t.Errorf("stored = %q", v)
	}
	if got := token(); got != "secret-token" {
		t.Errorf("read back = %q", got)
	}

	// A token saved in plain text before still works, and gets encrypted.
	st.db.Exec(`UPDATE user_settings SET paperless_token = 'old-token' WHERE user_id = ?`, u.ID)
	if got := token(); got != "old-token" {
		t.Errorf("plain token = %q", got)
	}
	if err := st.SealPaperlessTokens(ctx); err != nil {
		t.Fatal(err)
	}
	if v := stored(); !strings.HasPrefix(v, sealedPrefix) {
		t.Errorf("after sealing, stored = %q", v)
	}
	if got := token(); got != "old-token" {
		t.Errorf("sealed token = %q", got)
	}

	// With another secret the token is gone, rather than the settings.
	other, _ := secret.LoadKey("another secret", "")
	if s, err := New(st.db, other).Settings(ctx, u.ID); err != nil || s.PaperlessToken != "" {
		t.Errorf("with another key = %+v %v", s, err)
	}
}
