package i18n

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTranslate(t *testing.T) {
	for _, tc := range []struct {
		lang, key string
		args      Args
		want      string
	}{
		{"en", "server.notFound", nil, "Not found"},
		{"cs", "server.notFound", nil, "Nenalezeno"},
		{"cs", "login.continueWith", Args{"provider": "Pocket ID"}, "Pokračovat přes Pocket ID"},
		{"en", "reminder.title", Args{"count": 1}, "Maintenance due"},
		{"en", "reminder.title", Args{"count": 3}, "3 maintenance tasks due"},
		{"cs", "reminder.inDays", Args{"count": 1}, "za 1 den"},
		{"cs", "reminder.inDays", Args{"count": 3}, "za 3 dny"},
		{"cs", "reminder.inDays", Args{"count": 5}, "za 5 dní"},
		// Unknown languages and missing texts fall back to English.
		{"xx", "common.close", nil, "Close"},
		{"cs", "no.such.key", nil, "no.such.key"},
	} {
		if got := T(tc.lang, tc.key, tc.args); got != tc.want {
			t.Errorf("T(%s, %s, %v) = %q, want %q", tc.lang, tc.key, tc.args, got, tc.want)
		}
	}
}

func TestMatch(t *testing.T) {
	for header, want := range map[string]string{
		"cs-CZ,cs;q=0.9,en;q=0.8": "cs",
		"de-DE,en;q=0.5":          "en",
		"de":                      "",
		"":                        "",
	} {
		if got := Match(header); got != want {
			t.Errorf("Match(%q) = %q, want %q", header, got, want)
		}
	}
}

// Every key the server uses must exist in en.json.
func TestServerKeysExist(t *testing.T) {
	patterns := map[*regexp.Regexp]string{
		regexp.MustCompile(`"((?:server|signIn|reminder)\.\w+)"`): "%s",
		regexp.MustCompile(`invalid\("(\w+)"`):                    "server.%s",
		regexp.MustCompile(`msg\("(\w+)"\)`):                      "server.%s",
		regexp.MustCompile(`failLogin\(w, r, "(\w+)"`):            "signIn.%s",
	}
	root := filepath.Join("..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for re, format := range patterns {
			for _, m := range re.FindAllStringSubmatch(string(src), -1) {
				key := strings.Replace(format, "%s", m[1], 1)
				en := catalogs[Fallback]
				if _, ok := en[key]; !ok {
					if _, ok := en[key+"_other"]; !ok {
						t.Errorf("%s uses %q, which isn't in en.json", path, key)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
