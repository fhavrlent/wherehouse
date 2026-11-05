// Package homebox reads a HomeBox inventory (v0.27+, where locations and
// items are both "entities") through its API, for importing it.
package homebox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: time.Minute},
	}
}

// Error is a response HomeBox refused.
type Error struct {
	Status int
	Body   string
}

func (e *Error) Error() string {
	body := e.Body
	if len(body) > 300 {
		body = body[:300] + "…"
	}
	return fmt.Sprintf("HomeBox returned %d: %s", e.Status, strings.TrimSpace(body))
}

// Unauthorized reports whether HomeBox rejected the API key.
func (e *Error) Unauthorized() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.BaseURL + "/api/v1" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach HomeBox: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return &Error{Status: res.StatusCode, Body: string(b)}
	}
	return json.NewDecoder(io.LimitReader(res.Body, 32<<20)).Decode(out)
}

// User is the owner of the API key.
type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Self checks the API key and returns whose it is.
func (c *Client) Self(ctx context.Context) (*User, error) {
	var out struct {
		Item User `json:"item"`
	}
	if err := c.get(ctx, "/users/self", nil, &out); err != nil {
		return nil, err
	}
	return &out.Item, nil
}

// Node is a location or item in HomeBox's tree, with what is inside it.
type Node struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // "location" or "item"
	Children []Node `json:"children"`
}

// Tree returns every location and item, nested as they are in HomeBox.
func (c *Client) Tree(ctx context.Context) ([]Node, error) {
	var out []Node
	err := c.get(ctx, "/entities/tree", url.Values{"withItems": {"true"}}, &out)
	return out, err
}

// Entity is a HomeBox location or item, as much as is imported of it.
type Entity struct {
	ID               string       `json:"id"`
	AssetID          flexString   `json:"assetId"`
	Name             string       `json:"name"`
	Description      string       `json:"description"`
	Notes            string       `json:"notes"`
	Quantity         int          `json:"quantity"`
	Archived         bool         `json:"archived"`
	SerialNumber     string       `json:"serialNumber"`
	ModelNumber      string       `json:"modelNumber"`
	Manufacturer     string       `json:"manufacturer"`
	PurchaseDate     string       `json:"purchaseDate"`
	PurchasePrice    flexNumber   `json:"purchasePrice"`
	PurchaseFrom     string       `json:"purchaseFrom"`
	WarrantyExpires  string       `json:"warrantyExpires"`
	WarrantyDetails  string       `json:"warrantyDetails"`
	LifetimeWarranty bool         `json:"lifetimeWarranty"`
	Insured          bool         `json:"insured"`
	SoldDate         string       `json:"soldDate"`
	SoldPrice        flexNumber   `json:"soldPrice"`
	SoldTo           string       `json:"soldTo"`
	SoldNotes        string       `json:"soldNotes"`
	EntityType       *EntityType  `json:"entityType"`
	Tags             []Tag        `json:"tags"`
	Fields           []Field      `json:"fields"`
	Attachments      []Attachment `json:"attachments"`
}

// EntityType is a HomeBox 0.27 kind of entity, such as "Location" or a
// type people added.
type EntityType struct {
	Name       string `json:"name"`
	IsLocation bool   `json:"isLocation"`
}

type Tag struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Field is a custom field on an item.
type Field struct {
	Type         string     `json:"type"`
	Name         string     `json:"name"`
	TextValue    string     `json:"textValue"`
	NumberValue  flexNumber `json:"numberValue"`
	BooleanValue bool       `json:"booleanValue"`
}

// Attachment is a file or photo on an item.
type Attachment struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"` // photo, manual, receipt, warranty, attachment
	Primary  bool   `json:"primary"`
	MimeType string `json:"mimeType"`
	// External attachments are links, with no file to fetch.
	SourceType string `json:"sourceType"`
}

func (c *Client) Entity(ctx context.Context, id string) (*Entity, error) {
	var out Entity
	if err := c.get(ctx, "/entities/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Maintenance is an entry in an item's maintenance log.
type Maintenance struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Cost          flexNumber `json:"cost"`
	ScheduledDate string     `json:"scheduledDate"`
	CompletedDate string     `json:"completedDate"`
}

func (c *Client) Maintenance(ctx context.Context, id string) ([]Maintenance, error) {
	var out []Maintenance
	err := c.get(ctx, "/entities/"+url.PathEscape(id)+"/maintenance", nil, &out)
	return out, err
}

// LinkError is returned for an attachment that is a link to elsewhere.
type LinkError struct{ URL string }

func (e *LinkError) Error() string { return "attachment is a link to " + e.URL }

// OpenAttachment fetches an item's file or photo; the caller closes the body.
func (c *Client) OpenAttachment(ctx context.Context, itemID, attachmentID string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.BaseURL+"/api/v1/entities/"+url.PathEscape(itemID)+"/attachments/"+url.PathEscape(attachmentID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	// HomeBox may redirect to the file on its own host, which is followed. A
	// link attachment redirects to another host, which may not be reachable
	// from here, so it is reported instead.
	hc := *c.HTTP
	hc.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if !strings.EqualFold(next.URL.Host, req.URL.Host) {
			return http.ErrUseLastResponse
		}
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return nil
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach HomeBox: %w", err)
	}
	if res.StatusCode >= 300 && res.StatusCode < 400 && res.Header.Get("Location") != "" {
		res.Body.Close()
		loc, err := req.URL.Parse(res.Header.Get("Location"))
		if err != nil {
			return nil, err
		}
		return nil, &LinkError{URL: loc.String()}
	}
	if res.StatusCode >= 300 {
		defer res.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return nil, &Error{Status: res.StatusCode, Body: string(b)}
	}
	return res, nil
}

// Date turns one of HomeBox's dates (RFC 3339 or YYYY-MM-DD, with the zero
// time for none) into YYYY-MM-DD, or "".
func Date(s string) string {
	if len(s) < 10 || strings.HasPrefix(s, "0001-01-01") {
		return ""
	}
	if _, err := time.Parse("2006-01-02", s[:10]); err != nil {
		return ""
	}
	return s[:10]
}

// flexNumber reads a number HomeBox writes as a number or as text.
type flexNumber float64

func (f *flexNumber) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*f = 0
		return nil
	}
	*f = flexNumber(n)
	return nil
}

// flexString reads text HomeBox writes as text or as a number (asset IDs).
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	if string(b) == "null" {
		*f = ""
		return nil
	}
	*f = flexString(b)
	return nil
}

// AssetCode is the asset ID as HomeBox prints it on labels, e.g. "000-123",
// or "" for none.
func (e *Entity) AssetCode() string {
	s := strings.TrimSpace(string(e.AssetID))
	if s == "" || strings.Trim(s, "0-") == "" {
		return ""
	}
	if n, err := strconv.Atoi(s); err == nil {
		return fmt.Sprintf("%03d-%03d", n/1000, n%1000)
	}
	return s
}
