package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"bingo/internal/auth"
	"bingo/internal/config"
)

func TestNewProvider_nilWhenAuthDisabled(t *testing.T) {
	cfg := &config.Config{} // no OIDC fields
	p, err := auth.NewProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v, want nil", err)
	}
	if p != nil {
		t.Error("NewProvider() with no OIDC config = non-nil, want nil")
	}
}

func TestProvider_Middleware_nilProviderIsNoop(t *testing.T) {
	// A nil *Provider middleware must pass through without panicking.
	var p *auth.Provider
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, hasSession := auth.FromContext(r.Context())
		if hasSession {
			t.Error("nil provider middleware injected a session, want none")
		}
	})
	handler := p.Middleware(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if !called {
		t.Error("nil provider middleware did not call next handler")
	}
}

func TestFromContext_empty(t *testing.T) {
	_, ok := auth.FromContext(context.Background())
	if ok {
		t.Error("FromContext on empty context = true, want false")
	}
}

func TestProvider_LogoutURL_nilProviderReturnsEmpty(t *testing.T) {
	var p *auth.Provider
	if got := p.LogoutURL("id-token", "https://example.com/"); got != "" {
		t.Errorf("LogoutURL() on nil provider = %q, want empty", got)
	}
}

func TestProvider_LogoutURL(t *testing.T) {
	// Minimal fake OIDC discovery server advertising an end_session_endpoint,
	// so we can exercise the real discovery-parsing path in NewProvider.
	mux := http.NewServeMux()
	var issuer string
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issuer": "` + issuer + `",
			"authorization_endpoint": "` + issuer + `/oauth2/auth",
			"token_endpoint": "` + issuer + `/oauth2/token",
			"jwks_uri": "` + issuer + `/.well-known/jwks.json",
			"end_session_endpoint": "` + issuer + `/oauth2/sessions/logout",
			"response_types_supported": ["code"],
			"subject_types_supported": ["public"],
			"id_token_signing_alg_values_supported": ["RS256"]
		}`))
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys": []}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	issuer = ts.URL

	cfg := &config.Config{
		OIDCIssuerURL:    issuer,
		OIDCClientID:     "test-client",
		OIDCClientSecret: "test-secret",
		OIDCRedirectURL:  "https://app.example.com/auth/callback",
		SessionSecret:    "test-session-secret",
	}
	p, err := auth.NewProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if p == nil {
		t.Fatal("NewProvider() = nil, want non-nil")
	}

	got := p.LogoutURL("the-id-token", "https://app.example.com/")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("LogoutURL() returned unparsable URL %q: %v", got, err)
	}
	wantPath := "/oauth2/sessions/logout"
	if u.Path != wantPath {
		t.Errorf("LogoutURL() path = %q, want %q", u.Path, wantPath)
	}
	q := u.Query()
	if got := q.Get("id_token_hint"); got != "the-id-token" {
		t.Errorf("id_token_hint = %q, want %q", got, "the-id-token")
	}
	if got := q.Get("post_logout_redirect_uri"); got != "https://app.example.com/" {
		t.Errorf("post_logout_redirect_uri = %q, want %q", got, "https://app.example.com/")
	}
	if got := q.Get("client_id"); got != "test-client" {
		t.Errorf("client_id = %q, want %q", got, "test-client")
	}
}
