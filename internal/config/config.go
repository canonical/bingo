// Package config reads application configuration from environment variables.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const defaultMaxPasteSizeBytes int64 = 5 * 1024 * 1024 // 5 MiB

// Config holds all application configuration sourced from environment variables.
// OIDC fields are optional: when zero, authentication is disabled.
type Config struct {
	Port              string
	DatabaseURL       string
	MaxPasteSizeBytes int64
	BaseURL           string
	LogLevel          string

	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	SessionSecret    string

	WebDir string // WEB_DIR: path to web/dist; empty = disable static file serving
}

// BasePath returns the URL path component of BaseURL, with any trailing
// slash trimmed (e.g. "/bingo-tutorial-bingo" for a BaseURL of
// "https://host/bingo-tutorial-bingo"). Returns "" when BaseURL is empty,
// unparseable, or has no path — i.e. when the app is served from the
// domain root, as in local/non-charm runs or a subdomain-routed ingress.
//
// This exists because reverse proxies that route by path prefix (e.g.
// Traefik ingress-per-app in its default "path" mode) strip that prefix
// before forwarding requests to the app, so the app has no way to observe
// it from the request alone. paas-charm's go-framework extension injects
// the externally-visible URL (including any such prefix) as APP_BASE_URL,
// which is why BaseURL is the authoritative source for it here — the same
// source already used to build paste RawURL/URL and the OIDC redirect URL.
func (c *Config) BasePath() string {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(u.Path, "/")
}

// AuthEnabled reports whether OIDC authentication is fully configured.
// Returns true only when all four OIDC fields and SessionSecret are non-empty.
func (c *Config) AuthEnabled() bool {
	return c.OIDCIssuerURL != "" &&
		c.OIDCClientID != "" &&
		c.OIDCClientSecret != "" &&
		c.OIDCRedirectURL != "" &&
		c.SessionSecret != ""
}

// Load reads configuration from the environment, applying defaults where defined.
// Returns an error if any numeric variable is present but malformed.
func Load() (*Config, error) {
	// paas-charm's go-framework extension injects user-defined charm config
	// with an APP_ prefix; fall back to the unprefixed name for local/non-charm runs.
	maxSize, err := parseIntValue(firstEnv("APP_MAX_PASTE_SIZE_BYTES", "MAX_PASTE_SIZE_BYTES"), defaultMaxPasteSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("MAX_PASTE_SIZE_BYTES: %w", err)
	}

	baseURL := firstEnv("APP_BASE_URL", "BASE_URL")

	// OIDC config sources, in priority order:
	//  1. Plain OIDC_*/SESSION_SECRET env vars (standalone/non-charm deployments).
	//  2. The charm's `oauth` relation to Charmed Hydra, which paas_charm injects
	//     as APP_OAUTH_* (client id/secret/issuer/endpoints) and APP_SECRET_KEY
	//     (a charm-managed, peer-secret-stored session secret). The relation does
	//     not provide a redirect URL, so it is derived from the base URL and the
	//     app's fixed callback route.
	oidcIssuerURL := firstEnv("OIDC_ISSUER_URL", "APP_OAUTH_API_BASE_URL")
	oidcClientID := firstEnv("OIDC_CLIENT_ID", "APP_OAUTH_CLIENT_ID")
	oidcClientSecret := firstEnv("OIDC_CLIENT_SECRET", "APP_OAUTH_CLIENT_SECRET")
	oidcRedirectURL := os.Getenv("OIDC_REDIRECT_URL")
	if oidcRedirectURL == "" && oidcIssuerURL != "" && oidcClientID != "" && oidcClientSecret != "" && baseURL != "" {
		oidcRedirectURL = strings.TrimSuffix(baseURL, "/") + "/auth/callback"
	}
	sessionSecret := firstEnv("SESSION_SECRET", "APP_SECRET_KEY")

	cfg := &Config{
		Port:              envOrDefault("PORT", "8080"),
		DatabaseURL:       envOrDefault("POSTGRESQL_DB_CONNECT_STRING", os.Getenv("DATABASE_URL")),
		MaxPasteSizeBytes: maxSize,
		BaseURL:           baseURL,
		LogLevel:          valueOrDefault(firstEnv("APP_LOG_LEVEL", "LOG_LEVEL"), "info"),
		OIDCIssuerURL:     oidcIssuerURL,
		OIDCClientID:      oidcClientID,
		OIDCClientSecret:  oidcClientSecret,
		OIDCRedirectURL:   oidcRedirectURL,
		SessionSecret:     sessionSecret,
		WebDir:            firstEnv("APP_WEB_DIR", "WEB_DIR"),
	}

	// Validate OIDC config: either all-or-nothing, and SESSION_SECRET required.
	oidcCount := 0
	for _, v := range []string{cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.OIDCRedirectURL} {
		if v != "" {
			oidcCount++
		}
	}
	if oidcCount > 0 && oidcCount < 4 {
		return nil, fmt.Errorf("partial OIDC configuration: all four OIDC_* variables must be set together")
	}
	if oidcCount == 4 && cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required when OIDC is configured")
	}
	if oidcCount == 4 {
		if err := validateAbsoluteURL("OIDC issuer URL", cfg.OIDCIssuerURL); err != nil {
			return nil, err
		}
		if err := validateAbsoluteURL("OIDC redirect URL", cfg.OIDCRedirectURL); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// validateAbsoluteURL returns an error if raw is not a well-formed absolute
// URL with an http(s) scheme and a host, to catch OIDC misconfiguration
// (typos, missing scheme, stray whitespace) at startup rather than at the
// first login attempt.
func validateAbsoluteURL(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s %q is not a valid URL: %w", name, raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s %q must use http or https scheme", name, raw)
	}
	if u.Host == "" {
		return fmt.Errorf("%s %q must be an absolute URL with a host", name, raw)
	}
	return nil
}

// firstEnv returns the value of the first key that is set to a non-empty
// string, checking keys in order, or "" if none are set.
func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func envOrDefault(key, def string) string {
	return valueOrDefault(os.Getenv(key), def)
}

func valueOrDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func parseIntValue(raw string, def int64) (int64, error) {
	if raw == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q: %w", raw, err)
	}
	return n, nil
}
