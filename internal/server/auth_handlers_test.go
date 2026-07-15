package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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

// newFakeOIDCServer starts a minimal fake OIDC discovery server that
// advertises an end_session_endpoint, so tests can exercise RP-initiated
// logout without a real IdP.
func newFakeOIDCServer(t *testing.T) *httptest.Server {
	t.Helper()
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
	t.Cleanup(ts.Close)
	issuer = ts.URL
	return ts
}

func TestLogout_redirectsToIdPEndSessionEndpoint(t *testing.T) {
	idp := newFakeOIDCServer(t)
	cfg := &config.Config{
		OIDCIssuerURL:    idp.URL,
		OIDCClientID:     "test-client",
		OIDCClientSecret: "test-secret",
		OIDCRedirectURL:  "https://app.example.com/auth/callback",
		SessionSecret:    "test-session-secret",
	}
	provider, err := auth.NewProvider(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	ts := newTestServerWithAuth(t, provider)

	// Set a session cookie carrying an ID token, as handleCallback would.
	rec := httptest.NewRecorder()
	if err := provider.SetSession(rec, 1, "user-sub", "user@example.com", "the-id-token"); err != nil {
		t.Fatalf("SetSession() error = %v", err)
	}
	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "bingo_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("SetSession() did not set bingo_session cookie")
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/auth/logout", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.AddCookie(sessionCookie)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /auth/logout: %v", err)
	}
	defer resp.Body.Close()

	loc := resp.Header.Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("Location header %q not parsable: %v", loc, err)
	}
	if got, want := u.Scheme+"://"+u.Host+u.Path, idp.URL+"/oauth2/sessions/logout"; got != want {
		t.Errorf("logout Location = %q, want endpoint %q", got, want)
	}
	q := u.Query()
	if got := q.Get("id_token_hint"); got != "the-id-token" {
		t.Errorf("id_token_hint = %q, want %q", got, "the-id-token")
	}
	if got := q.Get("post_logout_redirect_uri"); got != "https://example.com/" {
		t.Errorf("post_logout_redirect_uri = %q, want %q", got, "https://example.com/")
	}
}
