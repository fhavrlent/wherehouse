package store

import (
	"context"
	"testing"
	"time"
)

// TestChangesPages reads the change feed from the start and checks every
// change comes once, however many changed in one second.
func TestChangesPages(t *testing.T) {
	st, _ := newTestStore(t)
	ctx := context.Background()
	u, err := st.UpsertUser(ctx, UpsertUser{Issuer: "test", Subject: "1"})
	if err != nil {
		t.Fatal(err)
	}
	hh, err := st.CurrentHousehold(ctx, u.ID, "Home")
	if err != nil {
		t.Fatal(err)
	}
	first, err := st.CreateThing(ctx, hh, u.ID, ThingInput{Name: "First"})
	if err != nil {
		t.Fatal(err)
	}
	st.db.Exec(`UPDATE things SET updated_at = '2026-01-01T10:00:00Z' WHERE id = ?`, first.ID)
	var kind int64
	if err := st.db.QueryRow(`SELECT kind_id FROM things WHERE id = ?`, first.ID).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	// 600 things in each of three seconds, then 1500 in one.
	add := func(at string, n int) {
		for range n {
			if _, err := st.db.Exec(`INSERT INTO things (household_id, kind_id, name, updated_at) VALUES (?, ?, 'x', ?)`,
				hh, kind, at); err != nil {
				t.Fatal(err)
			}
		}
	}
	add("2026-01-01T10:00:00Z", 599)
	add("2026-01-01T10:00:01Z", 600)
	add("2026-01-01T10:00:02Z", 600)
	add("2026-01-01T11:00:00Z", 1500)
	for id, at := range map[int64]string{90001: "2026-01-01T10:00:01Z", 90002: "2026-01-01T10:00:05Z", 90003: "2026-01-01T11:00:00Z"} {
		st.db.Exec(`INSERT INTO deleted_things (household_id, thing_id, deleted_at) VALUES (?, ?, ?)`, hh, id, at)
	}

	seen := map[int64]int{}
	deleted := map[int64]int{}
	var untils []string
	since := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	for page := 0; ; page++ {
		if page == 10 {
			t.Fatalf("still more after 10 pages: %v", untils)
		}
		ch, err := st.ChangesSince(ctx, hh, since)
		if err != nil {
			t.Fatal(err)
		}
		for _, th := range ch.Things {
			seen[th.ID]++
		}
		for _, id := range ch.Deleted {
			deleted[id]++
		}
		untils = append(untils, ch.Until)
		if !ch.More {
			break
		}
		if since, err = time.Parse(time.RFC3339, ch.Until); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"2026-01-01T10:00:02Z", "2026-01-01T11:00:01Z"}
	if len(untils) != 3 || untils[0] != want[0] || untils[1] != want[1] {
		t.Errorf("pages until %v", untils)
	}
	if len(seen) != 3300 {
		t.Errorf("%d things changed, want 3300", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("thing %d came %d times", id, n)
		}
	}
	if len(deleted) != 3 || deleted[90001] != 1 || deleted[90002] != 1 || deleted[90003] != 1 {
		t.Errorf("deleted = %v", deleted)
	}
}
