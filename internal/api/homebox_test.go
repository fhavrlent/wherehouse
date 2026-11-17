package api

import "testing"

func TestImportURL(t *testing.T) {
	for u, ok := range map[string]bool{
		"http://homebox.lan:7745":       true,
		"https://example.com":           true,
		"https://example.com/homebox":   true,
		"ftp://example.com":             false,
		"example.com":                   false,
		"https://":                      false,
		"https://user:pass@example.com": false,
		"https://evil.com@example.com":  false,
		"https://example.com/?x=1":      false,
		"https://example.com#frag":      false,
		"https://exa mple.com":          false,
	} {
		if got := importURL.MatchString(u); got != ok {
			t.Errorf("importURL(%q) = %v, want %v", u, got, ok)
		}
	}
}
