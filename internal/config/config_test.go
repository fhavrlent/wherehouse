package config

import "testing"

func TestLoadReadsEveryDocumentedSetting(t *testing.T) {
	t.Setenv("OIDC_ISSUER", "https://id.example.com")
	t.Setenv("OIDC_CLIENT_ID", "wherehouse")
	t.Setenv("PAPERLESS_URL", "http://paperless:8000/")
	t.Setenv("PAPERLESS_PUBLIC_URL", "https://paperless.example.com")
	t.Setenv("PAPERLESS_TOKEN", "token")
	t.Setenv("WHEREHOUSE_NOTIFY_HOUR", "7")
	t.Setenv("WHEREHOUSE_NOTIFY_ALLOWED_HOSTS", "ntfy.example.com, smtp.example.com")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	p := c.Paperless
	if p.URL != "http://paperless:8000" || p.PublicURL != "https://paperless.example.com" || p.Token != "token" || p.Field != "Wherehouse" {
		t.Errorf("paperless = %+v", p)
	}
	if c.NotifyHour != 7 || len(c.NotifyAllowedHosts) != 2 {
		t.Errorf("notify hour %d, hosts %v", c.NotifyHour, c.NotifyAllowedHosts)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("OIDC_ISSUER", "https://id.example.com")
	t.Setenv("OIDC_CLIENT_ID", "wherehouse")
	t.Setenv("PAPERLESS_URL", "http://paperless:8000")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.NotifyHour != 9 || c.Paperless.PublicURL != "http://paperless:8000" {
		t.Errorf("notify hour %d, paperless %+v", c.NotifyHour, c.Paperless)
	}
}
