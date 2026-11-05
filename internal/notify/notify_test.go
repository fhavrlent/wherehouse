package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fhavrlent/wherehouse/internal/db"
	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestRemindersFireOncePerThreshold(t *testing.T) {
	var mu sync.Mutex
	var messages []string
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		messages = append(messages, string(b))
		mu.Unlock()
	}))
	defer hook.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	st := store.New(database)
	ctx := context.Background()

	// The first user is an admin, so any notification host is allowed.
	u, err := st.UpsertUser(ctx, store.UpsertUser{Issuer: "test", Subject: "1", Name: "T"})
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := st.Settings(ctx, u.ID)
	settings.NotifyURLs = "generic+" + hook.URL + "/hook"
	settings.NotifyDays = "7, 0"
	if err := st.SaveSettings(ctx, u.ID, settings); err != nil {
		t.Fatal(err)
	}

	date := func(days int) *string {
		d := time.Now().AddDate(0, 0, days).Format(store.DateLayout)
		return &d
	}
	hh, err := st.CurrentHousehold(ctx, u.ID, "Home")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct {
		item, task string
		due        *string
	}{
		{"Boiler", "Yearly service", date(5)},
		{"Water filter", "Replace cartridge", date(-2)},
		{"Bike", "Oil the chain", date(40)},
	} {
		it, err := st.CreateThing(ctx, hh, u.ID, store.ThingInput{Name: e.item})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddMaintenance(ctx, hh, u.ID, it.ID, store.MaintenanceInput{Title: e.task, NextDue: e.due}); err != nil {
			t.Fatal(err)
		}
	}

	n := New(st, time.Local, 0, "https://wherehouse.example.com", Policy{})
	for range 2 {
		if err := n.CheckAll(ctx); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(messages) != 1 {
		t.Fatalf("sent %d messages, want 1 (checks don't repeat reminders): %q", len(messages), messages)
	}
	msg := messages[0]
	for _, want := range []string{"Boiler", "Yearly service", "in 5 days", "Water filter", "2 days overdue", "https://wherehouse.example.com/i/"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q: %s", want, msg)
		}
	}
	if strings.Contains(msg, "Bike") {
		t.Errorf("reminded of something 40 days away: %s", msg)
	}
}
