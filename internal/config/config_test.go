package config_test

import (
	"testing"

	"bingo/internal/config"
)

func TestLoad_defaults(t *testing.T) {
	for _, k := range []string{
		"PORT", "DATABASE_URL", "BASE_URL", "LOG_LEVEL", "MAX_PASTE_SIZE_BYTES",
		"OIDC_ISSUER_URL", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET",
		"OIDC_REDIRECT_URL", "SESSION_SECRET",
	} {
		t.Setenv(k, "")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Port", cfg.Port, "8080"},
		{"LogLevel", cfg.LogLevel, "info"},
		{"MaxPasteSizeBytes", cfg.MaxPasteSizeBytes, int64(5242880)},
		{"DatabaseURL", cfg.DatabaseURL, ""},
		{"BaseURL", cfg.BaseURL, ""},
		{"OIDCIssuerURL", cfg.OIDCIssuerURL, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestLoad_fromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://localhost/bingo")
	t.Setenv("BASE_URL", "https://example.com")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("MAX_PASTE_SIZE_BYTES", "1048576")
	t.Setenv("OIDC_ISSUER_URL", "https://identity.canonical.com")
	t.Setenv("OIDC_CLIENT_ID", "bingo-test")
	t.Setenv("OIDC_CLIENT_SECRET", "s3cr3t")
	t.Setenv("OIDC_REDIRECT_URL", "https://example.com/auth/callback")
	t.Setenv("SESSION_SECRET", "supersecretkey")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Port", cfg.Port, "9090"},
		{"DatabaseURL", cfg.DatabaseURL, "postgres://localhost/bingo"},
		{"BaseURL", cfg.BaseURL, "https://example.com"},
		{"LogLevel", cfg.LogLevel, "debug"},
		{"MaxPasteSizeBytes", cfg.MaxPasteSizeBytes, int64(1048576)},
		{"OIDCIssuerURL", cfg.OIDCIssuerURL, "https://identity.canonical.com"},
		{"OIDCClientID", cfg.OIDCClientID, "bingo-test"},
		{"OIDCClientSecret", cfg.OIDCClientSecret, "s3cr3t"},
		{"OIDCRedirectURL", cfg.OIDCRedirectURL, "https://example.com/auth/callback"},
		{"SessionSecret", cfg.SessionSecret, "supersecretkey"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestLoad_invalidMaxPasteSize(t *testing.T) {
	t.Setenv("MAX_PASTE_SIZE_BYTES", "not-a-number")

	_, err := config.Load()
	if err == nil {
		t.Error("Load() with invalid MAX_PASTE_SIZE_BYTES should return error, got nil")
	}
}
