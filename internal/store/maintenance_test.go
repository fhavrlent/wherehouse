package store

import (
	"context"
	"testing"
)

// When someone else marks an entry done between it being read and written,
// marking it done is refused rather than adding a second next entry or
// overwriting their date.
func TestCompleteMaintenanceOnce(t *testing.T) {
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
	car, err := st.CreateThing(ctx, hh, u.ID, ThingInput{Name: "Car"})
	if err != nil {
		t.Fatal(err)
	}
	due := "2026-04-15"
	m, err := st.AddMaintenance(ctx, hh, u.ID, car.ID, MaintenanceInput{Title: "Swap tyres", NextDue: &due,
		Repeat: RepeatField{Set: true, Value: &Repeat{Every: 6, Unit: RepeatMonth}}})
	if err != nil {
		t.Fatal(err)
	}

	first := true
	afterCompleteRead = func() {
		if first {
			first = false
			if _, _, err := st.CompleteMaintenance(ctx, hh, u.ID, m.ID, DoneInput{DoneOn: "2026-04-17"}); err != nil {
				t.Errorf("the other person's = %v", err)
			}
		}
	}
	t.Cleanup(func() { afterCompleteRead = func() {} })
	if _, _, err := st.CompleteMaintenance(ctx, hh, u.ID, m.ID, DoneInput{DoneOn: "2026-04-18"}); err == nil {
		t.Error("marking done a second time went through")
	}
	log, err := maintenanceOf(ctx, st.db, car.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 2 || log[1].DoneOn == nil || *log[1].DoneOn != "2026-04-17" {
		t.Errorf("log = %+v", log)
	}
}
