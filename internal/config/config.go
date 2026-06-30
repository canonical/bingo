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
}

// Load reads configuration from the environment, applying defaults where defined.
// Returns an error if any numeric variable is present but malformed.
func Load() (*Config, error) {
	maxSize, err := parseIntEnv("MAX_PASTE_SIZE_BYTES", defaultMaxPasteSizeBytes)
	if err != nil {
		return nil, fmt.Errorf("MAX_PASTE_SIZE_BYTES: %w", err)
	}

	return &Config{
		Port:              envOrDefault("PORT", "8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		MaxPasteSizeBytes: maxSize,
		BaseURL:           os.Getenv("BASE_URL"),
		LogLevel:          envOrDefault("LOG_LEVEL", "info"),
		OIDCIssuerURL:     os.Getenv("OIDC_ISSUER_URL"),
		OIDCClientID:      os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:  os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:   os.Getenv("OIDC_REDIRECT_URL"),
		SessionSecret:     os.Getenv("SESSION_SECRET"),
	}, nil
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
