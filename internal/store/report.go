package store

import (
	"context"
	"sort"
	"time"
)

// Money is amounts by currency.
type Money map[string]float64

func (m Money) add(currency string, v float64) { m[currency] += v }

// Group is the value and count of things in one place, tag or kind.
type Group struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
	Key   string `json:"key,omitempty"`
	Value Money  `json:"value"`
	Count int    `json:"count"`
}

type Report struct {
	// Currency is the household's; things without one of their own count in it.
	Currency string `json:"currency"`
	// Value is what the things cost (price × quantity), Insured what the
	// insured ones did, by currency.
	Value   Money `json:"value"`
	Insured Money `json:"insured"`
	// Things counts the current things that aren't places; Priced those of
	// them with a price.
	Things int `json:"things"`
	Priced int `json:"priced"`
	// ByPlace is by what the things are in at the top of the report,
	// ByTag and ByKind as they say; biggest first.
	ByPlace []Group `json:"byPlace"`
	ByTag   []Group `json:"byTag"`
	ByKind  []Group `json:"byKind"`
	// WarrantyEnding are things whose warranty ends within 90 days.
	WarrantyEnding []Thing `json:"warrantyEnding"`
	// Sold is what things sold this year brought.
	Sold Money `json:"sold"`
}

// WarrantySoonDays is how far ahead a report lists warranties ending.
const WarrantySoonDays = 90

// Report sums up the current things of a household, or those inside in.
// Sold, archived things and templates count toward nothing but Sold.
func (s *Store) Report(ctx context.Context, hh int64, in *int64, loc *time.Location) (*Report, error) {
	h, err := s.GetHousehold(ctx, hh)
	if err != nil {
		return nil, err
	}
	f := Filter{}
	if in != nil {
		f = Filter{Parent: in, Inside: true}
	}
	things, err := s.ListThings(ctx, hh, f)
	if err != nil {
		return nil, err
	}
	r := &Report{Currency: h.Currency, Value: Money{}, Insured: Money{}, Sold: Money{},
		ByPlace: []Group{}, ByTag: []Group{}, ByKind: []Group{}, WarrantyEnding: []Thing{}}
	places, tags, kinds := map[int64]*Group{}, map[int64]*Group{}, map[int64]*Group{}
	group := func(m map[int64]*Group, id int64, g Group) *Group {
		if m[id] == nil {
			g.ID, g.Value = id, Money{}
			m[id] = &g
		}
		return m[id]
	}
	y, mo, d := time.Now().In(loc).Date()
	today := time.Date(y, mo, d, 0, 0, 0, 0, time.UTC)
	soon := today.AddDate(0, 0, WarrantySoonDays).Format(DateLayout)
	yearStart := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC).Format(DateLayout)
	// What the report's things are in at its top: the first step of their
	// path below in.
	depth := 0
	if in != nil {
		for _, t := range things {
			if t.ParentID != nil && *t.ParentID == *in {
				depth = len(t.Path)
				break
			}
		}
	}

	for _, t := range things {
		cur := t.Currency
		if cur == "" {
			cur = h.Currency
		}
		if t.SoldDate != nil {
			if t.SoldPrice != nil && *t.SoldDate >= yearStart {
				r.Sold.add(cur, *t.SoldPrice)
			}
			continue
		}
		if !t.IsPlace() {
			r.Things++
		}
		if t.WarrantyUntil != nil && !t.LifetimeWarranty && *t.WarrantyUntil >= today.Format(DateLayout) && *t.WarrantyUntil <= soon {
			r.WarrantyEnding = append(r.WarrantyEnding, t)
		}
		if t.PurchasePrice == nil {
			continue
		}
		v := *t.PurchasePrice * float64(t.Quantity)
		if !t.IsPlace() {
			r.Priced++
		}
		r.Value.add(cur, v)
		if t.Insured {
			r.Insured.add(cur, v)
		}
		if len(t.Path) > depth {
			p := t.Path[depth]
			g := group(places, p.ID, Group{Name: p.Name, Icon: p.Icon})
			g.Value.add(cur, v)
			g.Count++
		}
		for _, tag := range t.Tags {
			g := group(tags, tag.ID, Group{Name: tag.Name, Color: tag.Color})
			g.Value.add(cur, v)
			g.Count++
		}
		g := group(kinds, t.Kind.ID, Group{Name: t.Kind.Name, Icon: t.Kind.Icon, Key: t.Kind.Key})
		g.Value.add(cur, v)
		g.Count++
	}
	sorted := func(m map[int64]*Group) []Group {
		out := []Group{}
		for _, g := range m {
			out = append(out, *g)
		}
		sort.Slice(out, func(i, j int) bool {
			if a, b := out[i].Value[h.Currency], out[j].Value[h.Currency]; a != b {
				return a > b
			}
			return out[i].Name < out[j].Name
		})
		return out
	}
	r.ByPlace, r.ByTag, r.ByKind = sorted(places), sorted(tags), sorted(kinds)
	sort.Slice(r.WarrantyEnding, func(i, j int) bool { return *r.WarrantyEnding[i].WarrantyUntil < *r.WarrantyEnding[j].WarrantyUntil })
	return r, nil
}
