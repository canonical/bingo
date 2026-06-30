package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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
