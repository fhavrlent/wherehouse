package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// Of several people adding a kind of one name at once, one gets it.
func TestKindNameCheckedWithTheWrite(t *testing.T) {
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
	var wg sync.WaitGroup
	var added atomic.Int32
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.CreateKind(ctx, hh, KindInput{Name: "Shelf", Icon: "shapes", UniqueName: true}); err == nil {
				added.Add(1)
			}
		}()
	}
	wg.Wait()
	if n := added.Load(); n != 1 {
		t.Errorf("%d added", n)
	}
	// A backup with two kinds of one name still comes in.
	if _, err := st.CreateKind(ctx, hh, KindInput{Name: "Shelf", Icon: "shapes"}); err != nil {
		t.Errorf("unchecked = %v", err)
	}
}
