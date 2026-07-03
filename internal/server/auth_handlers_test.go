package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bingo/internal/auth"
	"bingo/internal/config"
	"bingo/internal/server"
)

// newTestServerWithAuth creates a test server with the given auth provider (may be nil).
func newTestServerWithAuth(t *testing.T, authProvider *auth.Provider) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		BaseURL:           "https://example.com",
		MaxPasteSizeBytes: 5 * 1024 * 1024,
	}
	var userRepo *auth.UserRepository
	if authProvider != nil {
		userRepo = new(auth.UserRepository) // zero-value; DB calls would fail but middleware tests don't reach them
	}
	srv := server.New(cfg, nil, defaultRepo(), authProvider, userRepo)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

func TestLogin_authDisabled(t *testing.T) {
	// nil provider → 403 with auth_disabled code
	ts := newTestServerWithAuth(t, nil)

	resp, err := http.Get(ts.URL + "/auth/login")
	if err != nil {
		t.Fatalf("GET /auth/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestCallback_authDisabled(t *testing.T) {
	ts := newTestServerWithAuth(t, nil)

	resp, err := http.Get(ts.URL + "/auth/callback?state=x&code=y")
	if err != nil {
		t.Fatalf("GET /auth/callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("callback status = %d, want 403", resp.StatusCode)
	}
}

func TestLogout_authDisabled(t *testing.T) {
	ts := newTestServerWithAuth(t, nil)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(ts.URL + "/auth/logout")
	if err != nil {
		t.Fatalf("GET /auth/logout: %v", err)
	}
	defer resp.Body.Close()

	// Even when auth disabled, logout clears cookies and redirects to /
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("logout status = %d, want 302 or 303", resp.StatusCode)
	}
}

func TestLogout_clearsCSRFCookie(t *testing.T) {
	ts := newTestServerWithAuth(t, nil)

	// Use a client that doesn't follow redirects so we can inspect the Set-Cookie header.
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(ts.URL + "/auth/logout")
	if err != nil {
		t.Fatalf("GET /auth/logout: %v", err)
	}
	defer resp.Body.Close()

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "csrf_token" {
			found = true
			if c.MaxAge != -1 {
				t.Errorf("csrf_token MaxAge = %d, want -1 (clear)", c.MaxAge)
			}
		}
	}
	if !found {
		t.Error("logout did not set a csrf_token cookie to clear it")
	}
}
