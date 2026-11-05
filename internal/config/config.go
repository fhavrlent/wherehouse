// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppURL  *url.URL
	Listen  string
	DataDir string
	// FilesDir holds uploaded attachments. It may live on network storage
	// (NFS/SMB); the database in DataDir must not.
	FilesDir    string
	MaxUploadMB int64
	SessionTTL  time.Duration
	Location    *time.Location
	// NotifyHour is the hour of the day maintenance reminders go out.
	NotifyHour int
	// NotifyAllowedHosts are the hosts non-admins may send notifications to
	// with self-hosted services (ntfy, Gotify, webhooks, SMTP, ...).
	NotifyAllowedHosts []string

	OIDC      OIDC
	Paperless Paperless

	// Secret derives the key that encrypts stored refresh tokens and
	// Paperless tokens. When empty, a key is generated in DataDir/secret.key.
	Secret string

	// DevLogin enables a password-less local login without an identity
	// provider. Only meant for development; never enable it in production.
	DevLogin bool
}

type OIDC struct {
	Issuer        string
	ClientID      string
	ClientSecret  string
	Scopes        []string
	ProviderName  string
	AdminGroups   []string
	AllowedGroups []string
	GroupsClaim   string
	AutoRedirect  bool
	// LogoutAtProvider also ends the session at the identity provider via
	// RP-initiated logout when the provider supports it.
	LogoutAtProvider bool
	// RecheckInterval is how often a session's groups are re-checked with
	// the identity provider (using a refresh token).
	RecheckInterval time.Duration
}

// Paperless is the household's Paperless-ngx. Each person connects with
// their own API token, so Paperless's own permissions apply.
type Paperless struct {
	URL       string
	PublicURL string
	// Token is used for admins only, when they have none of their own.
	Token string
	// Field is the URL custom field documents link back here with.
	Field string
}

func (c *Config) Secure() bool { return c.AppURL.Scheme == "https" }

func (c *Config) RedirectURL() string {
	return strings.TrimSuffix(c.AppURL.String(), "/") + "/auth/callback"
}

func Load() (*Config, error) {
	var errs []error

	rawURL := env("WHEREHOUSE_APP_URL", "http://localhost:8080")
	appURL, err := url.Parse(rawURL)
	if err != nil || appURL.Scheme == "" || appURL.Host == "" {
		errs = append(errs, fmt.Errorf("WHEREHOUSE_APP_URL must be an absolute URL, got %q", rawURL))
		appURL = &url.URL{Scheme: "http", Host: "localhost:8080"}
	}

	loc := time.Local
	if tz := os.Getenv("TZ"); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		} else {
			errs = append(errs, fmt.Errorf("TZ: %w", err))
		}
	}

	c := &Config{
		AppURL:      appURL,
		Listen:      env("WHEREHOUSE_LISTEN", ":8080"),
		DataDir:     env("WHEREHOUSE_DATA_DIR", "./data"),
		MaxUploadMB: int64(envInt("WHEREHOUSE_MAX_UPLOAD_MB", 50, &errs)),
		SessionTTL:  time.Duration(envInt("WHEREHOUSE_SESSION_DAYS", 30, &errs)) * 24 * time.Hour,
		Location:    loc,
		DevLogin:    envBool("WHEREHOUSE_DEV_LOGIN", false, &errs),
		Secret:      os.Getenv("WHEREHOUSE_SECRET"),
		NotifyHour:  envInt("WHEREHOUSE_NOTIFY_HOUR", 9, &errs),

		NotifyAllowedHosts: list(os.Getenv("WHEREHOUSE_NOTIFY_ALLOWED_HOSTS")),
		Paperless: Paperless{
			URL:       strings.TrimSuffix(os.Getenv("PAPERLESS_URL"), "/"),
			PublicURL: strings.TrimSuffix(os.Getenv("PAPERLESS_PUBLIC_URL"), "/"),
			Token:     os.Getenv("PAPERLESS_TOKEN"),
			Field:     env("PAPERLESS_FIELD", "Wherehouse"),
		},
		OIDC: OIDC{
			Issuer:           strings.TrimSuffix(os.Getenv("OIDC_ISSUER"), "/"),
			ClientID:         os.Getenv("OIDC_CLIENT_ID"),
			ClientSecret:     os.Getenv("OIDC_CLIENT_SECRET"),
			Scopes:           list(env("OIDC_SCOPES", "openid profile email groups offline_access")),
			ProviderName:     env("OIDC_PROVIDER_NAME", "SSO"),
			AdminGroups:      list(os.Getenv("OIDC_ADMIN_GROUPS")),
			AllowedGroups:    list(os.Getenv("OIDC_ALLOWED_GROUPS")),
			GroupsClaim:      env("OIDC_GROUPS_CLAIM", "groups"),
			AutoRedirect:     envBool("OIDC_AUTO_REDIRECT", false, &errs),
			LogoutAtProvider: envBool("OIDC_LOGOUT_AT_PROVIDER", false, &errs),
			RecheckInterval:  envDuration("OIDC_RECHECK_INTERVAL", time.Hour, &errs),
		},
	}
	c.FilesDir = env("WHEREHOUSE_FILES_DIR", filepath.Join(c.DataDir, "files"))
	if c.Paperless.PublicURL == "" {
		c.Paperless.PublicURL = c.Paperless.URL
	}
	if c.NotifyHour < 0 || c.NotifyHour > 23 {
		errs = append(errs, errors.New("WHEREHOUSE_NOTIFY_HOUR must be between 0 and 23"))
	}

	if c.OIDC.Issuer == "" && !c.DevLogin {
		errs = append(errs, errors.New("OIDC_ISSUER is required (or set WHEREHOUSE_DEV_LOGIN=true for local development)"))
	}
	if c.OIDC.Issuer != "" && c.OIDC.ClientID == "" {
		errs = append(errs, errors.New("OIDC_CLIENT_ID is required when OIDC_ISSUER is set"))
	}
	if !contains(c.OIDC.Scopes, "openid") {
		c.OIDC.Scopes = append([]string{"openid"}, c.OIDC.Scopes...)
	}

	return c, errors.Join(errs...)
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func envInt(key string, def int, errs *[]error) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be a number, got %q", key, v))
		return def
	}
	return n
}

func envDuration(key string, def time.Duration, errs *[]error) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil || d < time.Minute {
		*errs = append(*errs, fmt.Errorf("%s must be a duration of at least 1m, e.g. 1h or 30m, got %q", key, v))
		return def
	}
	return d
}

func envBool(key string, def bool, errs *[]error) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be true or false, got %q", key, v))
		return def
	}
	return b
}

// list splits a comma or space separated value.
func list(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })
}

func lowerAll(xs []string) []string {
	for i, x := range xs {
		xs[i] = strings.ToLower(x)
	}
	return xs
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
