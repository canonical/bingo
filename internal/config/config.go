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
	maxSize, err := parseIntEnv("MAX_PASTE_SIZE_BYTES", defaultMaxPasteSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("MAX_PASTE_SIZE_BYTES: %w", err)
	}

	cfg := &Config{
		Port:              envOrDefault("PORT", "8080"),
		DatabaseURL:       envOrDefault("POSTGRESQL_DB_CONNECT_STRING", os.Getenv("DATABASE_URL")),
		MaxPasteSizeBytes: maxSize,
		BaseURL:           os.Getenv("BASE_URL"),
		LogLevel:          envOrDefault("LOG_LEVEL", "info"),
		OIDCIssuerURL:     os.Getenv("OIDC_ISSUER_URL"),
		OIDCClientID:      os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:  os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:   os.Getenv("OIDC_REDIRECT_URL"),
		SessionSecret:     os.Getenv("SESSION_SECRET"),
		WebDir:            os.Getenv("WEB_DIR"),
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

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseIntEnv(key string, def int64) (int64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q: %w", raw, err)
	}
	return n, nil
}
