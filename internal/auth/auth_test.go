package auth

import (
	"net/url"
	"testing"

	"github.com/fhavrlent/wherehouse/internal/config"
)

func TestSafeReturn(t *testing.T) {
	// From the query string "?return=/%09/evil.example".
	q, _ := url.ParseQuery("return=/%09/evil.example")
	tab := q.Get("return")

	cases := map[string]string{
		"/":                         "/",
		"/w/12":                     "/w/12",
		"/?filter=expired&tag=Home": "/?filter=expired&tag=Home",
		"/settings#data":            "/settings#data",
		tab:                         "/",
		"/\n/evil.example":          "/",
		"/\r/evil.example":          "/",
		"/\x00/evil.example":        "/",
		"/\x7f":                     "/",
		"//evil.example":            "/",
		"/\\evil.example":           "/",
		"/\\/evil.example":          "/",
		"\\\\evil.example":          "/",
		"/%2F%2Fevil.example":       "/",
		"/%5Cevil.example":          "/",
		"/w/a%20b":                  "/w/a%20b",
		"https://evil.example":      "/",
		"evil.example":              "/",
		"javascript:alert(1)":       "/",
		"":                          "/",
	}
	for in, want := range cases {
		if got := safeReturn(in); got != want {
			t.Errorf("safeReturn(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAccess(t *testing.T) {
	a := &Auth{cfg: &config.Config{OIDC: config.OIDC{
		GroupsClaim:   "groups",
		AllowedGroups: []string{"household"},
		AdminGroups:   []string{"admins"},
	}}}
	claims := func(groups ...any) map[string]any { return map[string]any{"groups": groups} }

	if ok, _ := a.access(claims("other")); ok {
		t.Error("user outside allowed groups must be refused")
	}
	if ok, admin := a.access(claims("household")); !ok || admin == nil || *admin {
		t.Error("household member: allowed, not admin")
	}
	if ok, admin := a.access(claims("admins")); !ok || admin == nil || !*admin {
		t.Error("admin group implies access and admin")
	}
	if ok, _ := a.access(map[string]any{}); ok {
		t.Error("no groups claim must be refused when groups are required")
	}
}
