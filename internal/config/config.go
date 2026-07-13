// Package config reads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
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

	cfg := &Config{
		Port:              envOrDefault("PORT", "8080"),
		DatabaseURL:       envOrDefault("POSTGRESQL_DB_CONNECT_STRING", os.Getenv("DATABASE_URL")),
		MaxPasteSizeBytes: maxSize,
		BaseURL:           firstEnv("APP_BASE_URL", "BASE_URL"),
		LogLevel:          valueOrDefault(firstEnv("APP_LOG_LEVEL", "LOG_LEVEL"), "info"),
		OIDCIssuerURL:     os.Getenv("OIDC_ISSUER_URL"),
		OIDCClientID:      os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:  os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:   os.Getenv("OIDC_REDIRECT_URL"),
		SessionSecret:     os.Getenv("SESSION_SECRET"),
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

	return cfg, nil
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
