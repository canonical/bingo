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

func TestConfig_AuthEnabled_false(t *testing.T) {
	// No OIDC env vars set → auth disabled
	cfg := &config.Config{}
	if cfg.AuthEnabled() {
		t.Error("AuthEnabled() = true, want false when no OIDC vars set")
	}
}

func TestConfig_AuthEnabled_true(t *testing.T) {
	cfg := &config.Config{
		OIDCIssuerURL:    "https://identity.example.com",
		OIDCClientID:     "my-client",
		OIDCClientSecret: "s3cr3t",
		OIDCRedirectURL:  "https://paste.example.com/auth/callback",
		SessionSecret:    "a-long-enough-secret-value-here!",
	}
	if !cfg.AuthEnabled() {
		t.Error("AuthEnabled() = false, want true when all OIDC vars set")
	}
}

func TestLoad_partialOIDCReturnsError(t *testing.T) {
	// Only some OIDC vars set → error
	t.Setenv("OIDC_ISSUER_URL", "https://identity.example.com")
	t.Setenv("OIDC_CLIENT_ID", "my-client")
	// OIDC_CLIENT_SECRET and OIDC_REDIRECT_URL missing
	_, err := config.Load()
	if err == nil {
		t.Error("Load() with partial OIDC config: want error, got nil")
	}
}

func TestLoad_OIDCEnabledRequiresSessionSecret(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "https://identity.example.com")
	t.Setenv("OIDC_CLIENT_ID", "my-client")
	t.Setenv("OIDC_CLIENT_SECRET", "s3cr3t")
	t.Setenv("OIDC_REDIRECT_URL", "https://paste.example.com/auth/callback")
	// SESSION_SECRET not set
	_, err := config.Load()
	if err == nil {
		t.Error("Load() with OIDC enabled but no SESSION_SECRET: want error, got nil")
	}
}

func TestLoad_invalidOIDCIssuerURLReturnsError(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "not-a-url\n")
	t.Setenv("OIDC_CLIENT_ID", "my-client")
	t.Setenv("OIDC_CLIENT_SECRET", "s3cr3t")
	t.Setenv("OIDC_REDIRECT_URL", "https://paste.example.com/auth/callback")
	t.Setenv("SESSION_SECRET", "a-long-enough-secret-value-here!")
	_, err := config.Load()
	if err == nil {
		t.Error("Load() with malformed OIDC_ISSUER_URL: want error, got nil")
	}
}

func TestLoad_OIDCIssuerURLMissingSchemeReturnsError(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "identity.example.com")
	t.Setenv("OIDC_CLIENT_ID", "my-client")
	t.Setenv("OIDC_CLIENT_SECRET", "s3cr3t")
	t.Setenv("OIDC_REDIRECT_URL", "https://paste.example.com/auth/callback")
	t.Setenv("SESSION_SECRET", "a-long-enough-secret-value-here!")
	_, err := config.Load()
	if err == nil {
		t.Error("Load() with schemeless OIDC_ISSUER_URL: want error, got nil")
	}
}

func TestLoad_invalidOIDCRedirectURLReturnsError(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "https://identity.example.com")
	t.Setenv("OIDC_CLIENT_ID", "my-client")
	t.Setenv("OIDC_CLIENT_SECRET", "s3cr3t")
	t.Setenv("OIDC_REDIRECT_URL", "ftp://paste.example.com/auth/callback")
	t.Setenv("SESSION_SECRET", "a-long-enough-secret-value-here!")
	_, err := config.Load()
	if err == nil {
		t.Error("Load() with non-http(s) OIDC_REDIRECT_URL: want error, got nil")
	}
}

func TestConfig_BasePath(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"empty base URL", "", ""},
		{"domain root, no path", "https://bingo.example.com", ""},
		{"domain root with trailing slash", "https://bingo.example.com/", ""},
		{"path prefix", "https://traefik-ip/bingo-tutorial-bingo", "/bingo-tutorial-bingo"},
		{"path prefix with trailing slash", "https://traefik-ip/bingo-tutorial-bingo/", "/bingo-tutorial-bingo"},
		{"unparseable URL", "://not a url", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{BaseURL: tt.baseURL}
			if got := cfg.BasePath(); got != tt.want {
				t.Errorf("BasePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func clearOIDCEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"OIDC_ISSUER_URL", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET", "OIDC_REDIRECT_URL",
		"SESSION_SECRET", "BASE_URL", "APP_BASE_URL",
		"APP_OAUTH_API_BASE_URL", "APP_OAUTH_CLIENT_ID", "APP_OAUTH_CLIENT_SECRET",
		"APP_SECRET_KEY",
	} {
		t.Setenv(k, "")
	}
}

func TestLoad_oauthRelationEnvVarsFallback(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv("APP_BASE_URL", "https://bingo.example.com")
	t.Setenv("APP_OAUTH_API_BASE_URL", "https://traefik-ip/model-hydra")
	t.Setenv("APP_OAUTH_CLIENT_ID", "bingo-oauth-client")
	t.Setenv("APP_OAUTH_CLIENT_SECRET", "hydra-issued-secret")
	t.Setenv("APP_SECRET_KEY", "charm-managed-secret-key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"OIDCIssuerURL", cfg.OIDCIssuerURL, "https://traefik-ip/model-hydra"},
		{"OIDCClientID", cfg.OIDCClientID, "bingo-oauth-client"},
		{"OIDCClientSecret", cfg.OIDCClientSecret, "hydra-issued-secret"},
		{"OIDCRedirectURL", cfg.OIDCRedirectURL, "https://bingo.example.com/auth/callback"},
		{"SessionSecret", cfg.SessionSecret, "charm-managed-secret-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
	if !cfg.AuthEnabled() {
		t.Error("AuthEnabled() = false, want true when charm-provided oauth vars are fully set")
	}
}

func TestLoad_plainOIDCVarsTakePrecedenceOverCharmVars(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv("APP_BASE_URL", "https://ignored.example.com")
	t.Setenv("OIDC_ISSUER_URL", "https://plain-issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "plain-client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "plain-secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://plain.example.com/auth/callback")
	t.Setenv("SESSION_SECRET", "plain-session-secret")
	t.Setenv("APP_OAUTH_API_BASE_URL", "https://charm-issuer.example.com")
	t.Setenv("APP_OAUTH_CLIENT_ID", "charm-client-id")
	t.Setenv("APP_OAUTH_CLIENT_SECRET", "charm-secret")
	t.Setenv("APP_SECRET_KEY", "charm-session-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"OIDCIssuerURL", cfg.OIDCIssuerURL, "https://plain-issuer.example.com"},
		{"OIDCClientID", cfg.OIDCClientID, "plain-client-id"},
		{"OIDCClientSecret", cfg.OIDCClientSecret, "plain-secret"},
		{"OIDCRedirectURL", cfg.OIDCRedirectURL, "https://plain.example.com/auth/callback"},
		{"SessionSecret", cfg.SessionSecret, "plain-session-secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestLoad_oauthFallbackMissingBaseURLReturnsPartialError(t *testing.T) {
	clearOIDCEnv(t)
	// Charm-provided client vars present, but no base URL to derive a redirect
	// from and no explicit OIDC_REDIRECT_URL — redirect stays empty, so the
	// all-or-nothing OIDC validation must reject this as partial configuration.
	t.Setenv("APP_OAUTH_API_BASE_URL", "https://traefik-ip/model-hydra")
	t.Setenv("APP_OAUTH_CLIENT_ID", "bingo-oauth-client")
	t.Setenv("APP_OAUTH_CLIENT_SECRET", "hydra-issued-secret")
	t.Setenv("APP_SECRET_KEY", "charm-managed-secret-key")

	_, err := config.Load()
	if err == nil {
		t.Error("Load() with no base URL to derive a redirect from: want error, got nil")
	}
}

func TestLoad_noOIDCVarsAtAllAuthDisabled(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv("APP_BASE_URL", "https://bingo.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AuthEnabled() {
		t.Error("AuthEnabled() = true, want false when no OIDC vars (plain or charm) are set")
	}
}
