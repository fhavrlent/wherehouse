package store

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fhavrlent/wherehouse/internal/i18n"
)

// MaxName is the longest name of a thing, in characters.
const MaxName = 200

// MaxText is the longest description or notes, in characters.
const MaxText = 10_000

// MaxQuantity is the most pieces one thing can count.
const MaxQuantity = 99_999

// DateLayout is how dates are stored and sent.
const DateLayout = "2006-01-02"

// Ref names a thing, e.g. in another's path.
type Ref struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// Thing is anything in the inventory: a building, a room, a box, or what is
// in it. Any thing can hold other things; its kind says whether it is shown
// as a place.
type Thing struct {
	ID           int64   `json:"id"`
	ParentID     *int64  `json:"parentId"`
	Kind         KindRef `json:"kind"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Quantity     int     `json:"quantity"`
	Manufacturer string  `json:"manufacturer"`
	Model        string  `json:"model"`
	Serial       string  `json:"serial"`
	Notes        string  `json:"notes"`
	Purchase

	// PurchaseOwner is the app whose purchase and warranty details these are,
	// such as "warranteed"; "" when they are this app's own.
	PurchaseOwner string `json:"purchaseOwner"`

	Template   bool    `json:"template"`
	ArchivedAt *string `json:"archivedAt"`
	// Path is the things this one is in, outermost first.
	Path []Ref `json:"path"`
	// Contains is how many current things are directly inside.
	Contains int    `json:"contains"`
	Cover    *int64 `json:"cover"`
	Label    string `json:"label"`
	Tags     []Tag  `json:"tags"`
	// The rest are filled in for one thing, not in lists.
	Photos      []Photo       `json:"photos,omitempty"`
	Fields      []FieldValue  `json:"fields,omitempty"`
	Attachments []Attachment  `json:"attachments,omitempty"`
	Maintenance []Maintenance `json:"maintenance,omitempty"`
	Links       []Link        `json:"links,omitempty"`
	CreatedAt   string        `json:"createdAt"`
	UpdatedAt   string        `json:"updatedAt"`
}

// Purchase is what a thing cost, its warranty and insurance, and whether
// it was sold. Prices are of one piece.
type Purchase struct {
	PurchaseDate     *string  `json:"purchaseDate"`
	PurchasePrice    *float64 `json:"purchasePrice"`
	PurchaseFrom     string   `json:"purchaseFrom"`
	Currency         string   `json:"currency"`
	WarrantyUntil    *string  `json:"warrantyUntil"`
	LifetimeWarranty bool     `json:"lifetimeWarranty"`
	WarrantyDetails  string   `json:"warrantyDetails"`
	Insured          bool     `json:"insured"`
	SoldDate         *string  `json:"soldDate"`
	SoldPrice        *float64 `json:"soldPrice"`
	SoldTo           string   `json:"soldTo"`
	SoldNotes        string   `json:"soldNotes"`
}

// IsPlace reports whether the thing is shown as a place: its kind is a
// place kind, or it holds something.
func (t *Thing) IsPlace() bool { return t.Kind.IsPlace || t.Contains > 0 }

type ThingInput struct {
	ParentID *int64 `json:"parentId"`
	// KindID 0 is the built-in "thing" kind.
	KindID       int64  `json:"kindId"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Quantity     int    `json:"quantity"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Serial       string `json:"serial"`
	Notes        string `json:"notes"`
	Purchase
	// Tags are names, added as needed; nil keeps the thing's tags.
	Tags *[]string `json:"tags"`
	// Fields are values by field ID; fields left out keep their values.
	Fields map[int64]string `json:"fields"`
}

func validDate(d **string) error {
	if *d == nil {
		return nil
	}
	v := strings.TrimSpace(**d)
	if v == "" {
		*d = nil
		return nil
	}
	if _, err := time.Parse(DateLayout, v); err != nil {
		return invalid("date", nil)
	}
	*d = &v
	return nil
}

func (in *ThingInput) normalize() error {
	if in.Tags != nil {
		tags, err := normalizeTags(*in.Tags)
		if err != nil {
			return err
		}
		in.Tags = &tags
	}
	for _, p := range []*string{&in.Name, &in.Description, &in.Manufacturer, &in.Model, &in.Serial, &in.Notes,
		&in.PurchaseFrom, &in.WarrantyDetails, &in.SoldTo, &in.SoldNotes} {
		*p = strings.TrimSpace(*p)
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Quantity == 0 {
		in.Quantity = 1
	}
	for _, d := range []**string{&in.PurchaseDate, &in.WarrantyUntil, &in.SoldDate} {
		if err := validDate(d); err != nil {
			return err
		}
	}
	long := func(max int, xs ...string) bool {
		return slices.ContainsFunc(xs, func(x string) bool { return utf8.RuneCountInString(x) > max })
	}
	switch {
	case in.Name == "":
		return invalid("thingName", nil)
	case long(MaxName, in.Name, in.Manufacturer, in.Model, in.Serial, in.PurchaseFrom, in.SoldTo):
		return invalid("nameLong", i18n.Args{"max": MaxName})
	case long(MaxText, in.Description, in.Notes, in.WarrantyDetails, in.SoldNotes):
		return invalid("textLong", i18n.Args{"max": MaxText})
	case in.Quantity < 1 || in.Quantity > MaxQuantity:
		return invalid("quantity", i18n.Args{"max": MaxQuantity})
	case (in.PurchasePrice != nil && *in.PurchasePrice < 0) || (in.SoldPrice != nil && *in.SoldPrice < 0):
		return invalid("negativePrice", nil)
	case in.Currency != "" && !currencyRe(in.Currency):
		return invalid("currency", nil)
	}
	return nil
}

func currencyRe(c string) bool {
	if len(c) != 3 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// ---- the tree ----

type node struct {
	id      int64
	parent  *int64
	name    string
	icon    string
	isPlace bool
}

// tree is every thing in a household by ID, for working out paths. A
// household has at most some thousands of things, so loading them is cheap.
type tree map[int64]*node

func (s *Store) loadTree(ctx context.Context, q queryer, hh int64) (tree, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT t.id, t.parent_id, t.name, k.icon, k.is_place FROM things t JOIN kinds k ON k.id = t.kind_id
		WHERE t.household_id = ?`, hh)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t := tree{}
	for rows.Next() {
		var n node
		var parent sql.NullInt64
		if err := rows.Scan(&n.id, &parent, &n.name, &n.icon, &n.isPlace); err != nil {
			return nil, err
		}
		if parent.Valid {
			n.parent = &parent.Int64
		}
		t[n.id] = &n
	}
	return t, rows.Err()
}

// path is the things id is in, outermost first, without id itself.
func (t tree) path(id *int64) []Ref {
	out := []Ref{}
	if id == nil {
		return out
	}
	seen := map[int64]bool{}
	for n := t[*id]; n != nil && !seen[n.id]; {
		seen[n.id] = true
		out = append(out, Ref{ID: n.id, Name: n.name, Icon: n.icon})
		if n.parent == nil {
			break
		}
		n = t[*n.parent]
	}
	slices.Reverse(out)
	return out
}

func (t tree) pathText(id *int64) string {
	var names []string
	for _, r := range t.path(id) {
		names = append(names, r.Name)
	}
	return strings.Join(names, " ")
}

// inside is id and everything inside it, at any depth.
func (t tree) inside(id int64) []int64 {
	children := map[int64][]int64{}
	for _, n := range t {
		if n.parent != nil {
			children[*n.parent] = append(children[*n.parent], n.id)
		}
	}
	out := []int64{id}
	for i := 0; i < len(out); i++ {
		out = append(out, children[out[i]]...)
	}
	return out
}

// ---- reading ----

const thingColumns = `t.id, t.parent_id, k.id, k.key, k.name, k.icon, k.is_place,
	t.name, t.description, t.quantity, t.manufacturer, t.model, t.serial, t.notes,
	t.purchase_date, t.purchase_price, t.purchase_from, t.currency, t.warranty_until, t.lifetime_warranty,
	t.warranty_details, t.insured, t.sold_date, t.sold_price, t.sold_to, t.sold_notes,
	t.purchase_owner, t.is_template, t.archived_at, t.created_at, t.updated_at,
	(SELECT COUNT(*) FROM things c WHERE c.parent_id = t.id AND c.archived_at IS NULL AND c.is_template = 0),
	(SELECT ph.id FROM photos ph WHERE ph.thing_id = t.id ORDER BY ph.position, ph.id LIMIT 1),
	COALESCE((SELECT l.code FROM labels l WHERE l.thing_id = t.id), '')`

const thingFrom = ` FROM things t JOIN kinds k ON k.id = t.kind_id `

func scanThing(sc interface{ Scan(...any) error }) (*Thing, error) {
	var t Thing
	var parent, cover sql.NullInt64
	var key, purchaseDate, warranty, soldDate, archived sql.NullString
	var price, soldPrice sql.NullFloat64
	if err := sc.Scan(&t.ID, &parent, &t.Kind.ID, &key, &t.Kind.Name, &t.Kind.Icon, &t.Kind.IsPlace,
		&t.Name, &t.Description, &t.Quantity, &t.Manufacturer, &t.Model, &t.Serial, &t.Notes,
		&purchaseDate, &price, &t.PurchaseFrom, &t.Currency, &warranty, &t.LifetimeWarranty,
		&t.WarrantyDetails, &t.Insured, &soldDate, &soldPrice, &t.SoldTo, &t.SoldNotes,
		&t.PurchaseOwner, &t.Template, &archived, &t.CreatedAt, &t.UpdatedAt, &t.Contains, &cover, &t.Label); err != nil {
		return nil, err
	}
	str := func(n sql.NullString) *string {
		if n.Valid {
			return &n.String
		}
		return nil
	}
	num := func(n sql.NullFloat64) *float64 {
		if n.Valid {
			return &n.Float64
		}
		return nil
	}
	if parent.Valid {
		t.ParentID = &parent.Int64
	}
	if cover.Valid {
		t.Cover = &cover.Int64
	}
	t.Kind.Key = key.String
	t.PurchaseDate, t.WarrantyUntil, t.SoldDate, t.ArchivedAt = str(purchaseDate), str(warranty), str(soldDate), str(archived)
	t.PurchasePrice, t.SoldPrice = num(price), num(soldPrice)
	return &t, nil
}

// Filter picks which things ListThings returns. Archived things and
// templates are left out unless asked for.
type Filter struct {
	// Parent lists what is directly in a thing; Top what is in nothing.
	Parent *int64
	Top    bool
	// Inside also lists what is inside the things inside Parent.
	Inside bool
	// Unplaced lists things that aren't in anything and aren't places.
	Unplaced bool
	// Places lists only things shown as places (place kinds, and things
	// holding something).
	Places    bool
	TagID     *int64
	Archived  bool
	Templates bool
	// Recent orders by last change, newest first, instead of places first
	// and then by name.
	Recent bool
	Limit  int
}

func (s *Store) ListThings(ctx context.Context, hh int64, f Filter) ([]Thing, error) {
	t, err := s.loadTree(ctx, s.db, hh)
	if err != nil {
		return nil, err
	}
	where := []string{"t.household_id = ?"}
	args := []any{hh}
	hasChildren := "EXISTS (SELECT 1 FROM things c WHERE c.parent_id = t.id AND c.archived_at IS NULL AND c.is_template = 0)"
	switch {
	case f.Unplaced:
		where = append(where, "t.parent_id IS NULL AND k.is_place = 0 AND NOT "+hasChildren)
	case f.Parent != nil && f.Inside:
		ids := t.inside(*f.Parent)[1:]
		if len(ids) == 0 {
			return []Thing{}, nil
		}
		where = append(where, "t.id IN ("+placeholders(len(ids))+")")
		for _, id := range ids {
			args = append(args, id)
		}
	case f.Parent != nil:
		where = append(where, "t.parent_id = ?")
		args = append(args, *f.Parent)
	case f.Top:
		where = append(where, "t.parent_id IS NULL")
	}
	if f.Places {
		where = append(where, "(k.is_place = 1 OR "+hasChildren+")")
	}
	if f.TagID != nil {
		where = append(where, "t.id IN (SELECT thing_id FROM thing_tags WHERE tag_id = ?)")
		args = append(args, *f.TagID)
	}
	if f.Archived {
		where = append(where, "t.archived_at IS NOT NULL")
	} else {
		where = append(where, "t.archived_at IS NULL")
	}
	if f.Templates {
		where = append(where, "t.is_template = 1")
	} else {
		where = append(where, "t.is_template = 0")
	}
	q := `SELECT ` + thingColumns + thingFrom + ` WHERE ` + strings.Join(where, " AND ")
	if f.Recent {
		q += ` ORDER BY t.updated_at DESC, t.id DESC`
	} else {
		q += ` ORDER BY (k.is_place = 1 OR ` + hasChildren + `) DESC, t.name COLLATE NOCASE, t.id`
	}
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	return s.queryThings(ctx, t, q, args...)
}

func placeholders(n int) string { return strings.Repeat("?,", n-1) + "?" }

func (s *Store) queryThings(ctx context.Context, t tree, q string, args ...any) ([]Thing, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := []Thing{}
	var ids []int64
	for rows.Next() {
		th, err := scanThing(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		th.Path = t.path(th.ParentID)
		out = append(out, *th)
		ids = append(ids, th.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	tags, err := tagsOf(ctx, s.db, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Tags = orEmpty(tags[out[i].ID])
	}
	return out, nil
}

func orEmpty[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}

// GetThing returns a thing with all its details.
func (s *Store) GetThing(ctx context.Context, hh, id int64) (*Thing, error) {
	th, err := scanThing(s.db.QueryRowContext(ctx, `SELECT `+thingColumns+thingFrom+` WHERE t.id = ? AND t.household_id = ?`, id, hh))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t, err := s.loadTree(ctx, s.db, hh)
	if err != nil {
		return nil, err
	}
	th.Path = t.path(th.ParentID)
	tags, err := tagsOf(ctx, s.db, []int64{id})
	if err != nil {
		return nil, err
	}
	th.Tags = orEmpty(tags[id])
	if th.Photos, err = s.photosOf(ctx, id); err != nil {
		return nil, err
	}
	if th.Fields, err = fieldsOf(ctx, s.db, id); err != nil {
		return nil, err
	}
	if th.Attachments, err = attachmentsOf(ctx, s.db, id); err != nil {
		return nil, err
	}
	if th.Maintenance, err = maintenanceOf(ctx, s.db, id); err != nil {
		return nil, err
	}
	if th.Links, err = linksOf(ctx, s.db, id); err != nil {
		return nil, err
	}
	return th, nil
}

// ---- writing ----

// owned checks that thing id is in household hh.
func owned(ctx context.Context, q queryer, hh, id int64) error {
	var one int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM things WHERE id = ? AND household_id = ?`, id, hh).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// checkParent makes sure thing id (0 for a new one) can go inside parent:
// parent is in the household and isn't id or something inside it.
func (s *Store) checkParent(ctx context.Context, tx *sql.Tx, hh, id int64, parent *int64) error {
	if parent == nil {
		return nil
	}
	t, err := s.loadTree(ctx, tx, hh)
	if err != nil {
		return err
	}
	if t[*parent] == nil {
		return invalid("parentMissing", nil)
	}
	if id == 0 {
		return nil
	}
	if *parent == id || slices.ContainsFunc(t.path(parent), func(r Ref) bool { return r.ID == id }) {
		return invalid("parentInside", nil)
	}
	return nil
}

func (s *Store) kindFor(ctx context.Context, q queryer, hh, kind int64) (int64, error) {
	if kind == 0 {
		err := q.QueryRowContext(ctx, `SELECT id FROM kinds WHERE household_id = ? AND key = 'thing'`, hh).Scan(&kind)
		return kind, err
	}
	var one int
	if err := q.QueryRowContext(ctx, `SELECT 1 FROM kinds WHERE id = ? AND household_id = ?`, kind, hh).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, invalid("kindMissing", nil)
		}
		return 0, err
	}
	return kind, nil
}

func (s *Store) CreateThing(ctx context.Context, hh, uid int64, in ThingInput) (*Thing, error) {
	return s.createThing(ctx, hh, uid, in, false)
}

func (s *Store) createThing(ctx context.Context, hh, uid int64, in ThingInput, template bool) (*Thing, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	var id int64
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.checkParent(ctx, tx, hh, 0, in.ParentID); err != nil {
			return err
		}
		kind, err := s.kindFor(ctx, tx, hh, in.KindID)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO things (household_id, parent_id, kind_id, name, description, quantity, manufacturer, model, serial, notes,
				purchase_date, purchase_price, purchase_from, currency, warranty_until, lifetime_warranty, warranty_details,
				insured, sold_date, sold_price, sold_to, sold_notes, is_template, created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			hh, in.ParentID, kind, in.Name, in.Description, in.Quantity, in.Manufacturer, in.Model, in.Serial, in.Notes,
			in.PurchaseDate, in.PurchasePrice, in.PurchaseFrom, in.Currency, in.WarrantyUntil, in.LifetimeWarranty, in.WarrantyDetails,
			in.Insured, in.SoldDate, in.SoldPrice, in.SoldTo, in.SoldNotes, template, uid, now(), now())
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		if err := setDetails(ctx, tx, hh, id, in); err != nil {
			return err
		}
		return s.reindex(ctx, tx, hh, id)
	})
	if err != nil {
		return nil, err
	}
	return s.GetThing(ctx, hh, id)
}

func setDetails(ctx context.Context, tx *sql.Tx, hh, id int64, in ThingInput) error {
	if in.Tags != nil {
		if err := setThingTags(ctx, tx, hh, id, *in.Tags); err != nil {
			return err
		}
	}
	return setThingFields(ctx, tx, hh, id, in.Fields)
}

// UpdateThing changes a thing, including what it is in. Everything inside
// it is reindexed, as their paths change with it.
func (s *Store) UpdateThing(ctx context.Context, hh, id int64, in ThingInput) (*Thing, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := owned(ctx, tx, hh, id); err != nil {
			return err
		}
		if err := keepOwnedPurchase(ctx, tx, id, &in); err != nil {
			return err
		}
		if err := s.checkParent(ctx, tx, hh, id, in.ParentID); err != nil {
			return err
		}
		kind, err := s.kindFor(ctx, tx, hh, in.KindID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE things SET parent_id = ?, kind_id = ?, name = ?, description = ?, quantity = ?, manufacturer = ?, model = ?,
				serial = ?, notes = ?, purchase_date = ?, purchase_price = ?, purchase_from = ?, currency = ?, warranty_until = ?,
				lifetime_warranty = ?, warranty_details = ?, insured = ?, sold_date = ?, sold_price = ?, sold_to = ?, sold_notes = ?,
				updated_at = ?
			WHERE id = ?`,
			in.ParentID, kind, in.Name, in.Description, in.Quantity, in.Manufacturer, in.Model, in.Serial, in.Notes,
			in.PurchaseDate, in.PurchasePrice, in.PurchaseFrom, in.Currency, in.WarrantyUntil, in.LifetimeWarranty,
			in.WarrantyDetails, in.Insured, in.SoldDate, in.SoldPrice, in.SoldTo, in.SoldNotes, now(), id); err != nil {
			return err
		}
		if err := setDetails(ctx, tx, hh, id, in); err != nil {
			return err
		}
		return s.reindexInside(ctx, tx, hh, id)
	})
	if err != nil {
		return nil, err
	}
	return s.GetThing(ctx, hh, id)
}

// keepOwnedPurchase keeps the purchase and warranty details another app
// owns as they are. Insurance and a sale stay this app's to change.
func keepOwnedPurchase(ctx context.Context, tx *sql.Tx, id int64, in *ThingInput) error {
	var owner string
	if err := tx.QueryRowContext(ctx, `SELECT purchase_owner FROM things WHERE id = ?`, id).Scan(&owner); err != nil || owner == "" {
		return err
	}
	var date, until sql.NullString
	var price sql.NullFloat64
	if err := tx.QueryRowContext(ctx, `SELECT purchase_date, purchase_price, purchase_from, currency,
		warranty_until, lifetime_warranty, warranty_details FROM things WHERE id = ?`, id).
		Scan(&date, &price, &in.PurchaseFrom, &in.Currency, &until, &in.LifetimeWarranty, &in.WarrantyDetails); err != nil {
		return err
	}
	in.PurchaseDate, in.WarrantyUntil, in.PurchasePrice = nil, nil, nil
	if date.Valid {
		in.PurchaseDate = &date.String
	}
	if until.Valid {
		in.WarrantyUntil = &until.String
	}
	if price.Valid {
		in.PurchasePrice = &price.Float64
	}
	return nil
}

// Move puts things, with everything inside them, inside parent (nil:
// nothing).
func (s *Store) Move(ctx context.Context, hh int64, ids []int64, parent *int64) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			if err := owned(ctx, tx, hh, id); err != nil {
				return err
			}
			if err := s.checkParent(ctx, tx, hh, id, parent); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE things SET parent_id = ?, updated_at = ? WHERE id = ?`, parent, now(), id); err != nil {
				return err
			}
			if err := s.reindexInside(ctx, tx, hh, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// Archive hides a thing from lists without deleting it, e.g. one that was
// thrown away. Things inside it stay where they are.
func (s *Store) Archive(ctx context.Context, hh, id int64, archived bool) (*Thing, error) {
	var at any
	if archived {
		at = now()
	}
	res, err := s.db.ExecContext(ctx, `UPDATE things SET archived_at = ?, updated_at = ? WHERE id = ? AND household_id = ?`,
		at, now(), id, hh)
	if err != nil {
		return nil, err
	}
	if err := affected(res); err != nil {
		return nil, err
	}
	return s.GetThing(ctx, hh, id)
}

// DeleteThing deletes a thing that holds nothing, and returns the files of
// its photos and documents for the caller to remove.
func (s *Store) DeleteThing(ctx context.Context, hh, id int64) ([]string, error) {
	var files []string
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := owned(ctx, tx, hh, id); err != nil {
			return err
		}
		var children int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM things WHERE parent_id = ?`, id).Scan(&children); err != nil {
			return err
		}
		if children > 0 {
			return invalid("thingNotEmpty", nil)
		}
		var err error
		if files, err = thingFiles(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM things WHERE id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM search WHERE ref = ?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO deleted_things (household_id, thing_id, deleted_at) VALUES (?, ?, ?)`,
			hh, id, now()); err != nil {
			return err
		}
		return dropUnusedTags(ctx, tx, hh)
	})
	return files, err
}

// thingFiles lists the stored files of a thing's photos and documents.
func thingFiles(ctx context.Context, tx *sql.Tx, id int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT file FROM photos WHERE thing_id = ?
		UNION ALL SELECT file FROM attachments WHERE thing_id = ? AND file <> ''`, id, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ---- helpers ----

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func affected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// touch marks a thing as changed now.
func touch(ctx context.Context, tx *sql.Tx, id int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE things SET updated_at = ? WHERE id = ?`, now(), id)
	return err
}

// CopyInput says how a thing is copied. Its name, details, purchase,
// tags and custom fields are copied; labels, documents, maintenance, links,
// what is inside it and a sale are not.
type CopyInput struct {
	// Name "" keeps the name.
	Name string `json:"name"`
	// ParentID is where the copy goes, unless SameParent puts it beside the
	// original.
	ParentID   *int64 `json:"parentId"`
	SameParent bool   `json:"sameParent"`
	// Template makes the copy a template, which is in nothing.
	Template bool `json:"template"`
}

// CopyThing copies a thing, or makes a template of it, or a thing from a
// template.
func (s *Store) CopyThing(ctx context.Context, hh, uid, id int64, in CopyInput) (*Thing, error) {
	src, err := s.GetThing(ctx, hh, id)
	if err != nil {
		return nil, err
	}
	tags := []string{}
	for _, t := range src.Tags {
		tags = append(tags, t.Name)
	}
	fields := map[int64]string{}
	for _, f := range src.Fields {
		fields[f.FieldID] = f.Value
	}
	p := src.Purchase
	p.SoldDate, p.SoldPrice, p.SoldTo, p.SoldNotes = nil, nil, "", ""
	t := ThingInput{
		ParentID: in.ParentID, KindID: src.Kind.ID, Name: src.Name, Description: src.Description, Quantity: src.Quantity,
		Manufacturer: src.Manufacturer, Model: src.Model, Serial: src.Serial, Notes: src.Notes, Purchase: p,
		Tags: &tags, Fields: fields,
	}
	if strings.TrimSpace(in.Name) != "" {
		t.Name = in.Name
	}
	if in.SameParent {
		t.ParentID = src.ParentID
	}
	if in.Template {
		t.ParentID = nil
	}
	return s.createThing(ctx, hh, uid, t, in.Template)
}
