package server_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	josejwt "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

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
	if sessErr := provider.SetSession(rec, 1, "user-sub", "user@example.com", "the-id-token"); sessErr != nil {
		t.Fatalf("SetSession() error = %v", sessErr)
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

// newFakeIdPWithToken starts a fake OIDC IdP exposing discovery, JWKS, and a
// token endpoint, so tests can exercise the full /auth/callback code exchange
// against a real (locally signed) ID token. The "code" query/form value sent
// to the token endpoint selects its behaviour: "bad-code" simulates a
// rejected exchange (HTTP 400); anything else returns a well-formed token
// response for the given sub/email.
func newFakeIdPWithToken(t *testing.T, clientID, sub, email string) *httptest.Server {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	const kid = "test-key"

	mux := http.NewServeMux()
	var issuer string
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issuer": "` + issuer + `",
			"authorization_endpoint": "` + issuer + `/oauth2/auth",
			"token_endpoint": "` + issuer + `/oauth2/token",
			"jwks_uri": "` + issuer + `/.well-known/jwks.json",
			"response_types_supported": ["code"],
			"subject_types_supported": ["public"],
			"id_token_signing_alg_values_supported": ["RS256"]
		}`))
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		jwks := josejwt.JSONWebKeySet{Keys: []josejwt.JSONWebKey{
			{Key: &priv.PublicKey, KeyID: kid, Algorithm: "RS256", Use: "sig"},
		}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("code") == "bad-code" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		signer, err := josejwt.NewSigner(
			josejwt.SigningKey{Algorithm: josejwt.RS256, Key: priv},
			(&josejwt.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		stdClaims := jwt.Claims{
			Issuer:   issuer,
			Subject:  sub,
			Audience: jwt.Audience{clientID},
			Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		}
		emailClaims := struct {
			Email string `json:"email"`
		}{Email: email}
		rawIDToken, err := jwt.Signed(signer).Claims(stdClaims).Claims(emailClaims).Serialize()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     rawIDToken,
		})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	issuer = ts.URL
	return ts
}

// newCallbackTestServer builds a Server wired with a real *auth.Provider
// (pointed at idp) and a real *auth.UserRepository (backed by the shared
// test DB), so /auth/callback can be exercised end to end.
func newCallbackTestServer(t *testing.T, idp *httptest.Server, clientID string) *httptest.Server {
	t.Helper()
	db := requireTestDB(t)
	cfg := &config.Config{
		BaseURL:          "https://example.com",
		OIDCIssuerURL:    idp.URL,
		OIDCClientID:     clientID,
		OIDCClientSecret: "test-secret",
		OIDCRedirectURL:  "https://app.example.com/auth/callback",
		SessionSecret:    "test-session-secret",
	}
	provider, err := auth.NewProvider(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	userRepo := auth.NewUserRepository(db)
	srv := server.New(cfg, db, defaultRepo(), provider, userRepo)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

func callbackClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func TestCallback_success(t *testing.T) {
	idp := newFakeIdPWithToken(t, "test-client", "sub|callback-test", "callback@example.com")
	ts := newCallbackTestServer(t, idp, "test-client")

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/auth/callback?state=matching-state&code=good-code", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "matching-state"})

	resp, err := callbackClient().Do(req)
	if err != nil {
		t.Fatalf("GET /auth/callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/" {
		t.Errorf("Location = %q, want %q", got, "/")
	}

	var sessionSet, csrfSet, stateCleared bool
	for _, c := range resp.Cookies() {
		switch c.Name {
		case "bingo_session":
			sessionSet = true
		case "csrf_token":
			csrfSet = true
		case "oidc_state":
			stateCleared = c.MaxAge < 0
		}
	}
	if !sessionSet {
		t.Error("successful callback did not set bingo_session cookie")
	}
	if !csrfSet {
		t.Error("successful callback did not set csrf_token cookie")
	}
	if !stateCleared {
		t.Error("successful callback did not clear the oidc_state cookie")
	}
}

func TestCallback_stateMismatch(t *testing.T) {
	idp := newFakeIdPWithToken(t, "test-client", "sub|x", "x@example.com")
	ts := newCallbackTestServer(t, idp, "test-client")

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/auth/callback?state=expected&code=good-code", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "different"})

	resp, err := callbackClient().Do(req)
	if err != nil {
		t.Fatalf("GET /auth/callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCallback_missingCode(t *testing.T) {
	idp := newFakeIdPWithToken(t, "test-client", "sub|x", "x@example.com")
	ts := newCallbackTestServer(t, idp, "test-client")

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/auth/callback?state=matching-state", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "matching-state"})

	resp, err := callbackClient().Do(req)
	if err != nil {
		t.Fatalf("GET /auth/callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCallback_exchangeFailure(t *testing.T) {
	idp := newFakeIdPWithToken(t, "test-client", "sub|x", "x@example.com")
	ts := newCallbackTestServer(t, idp, "test-client")

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/auth/callback?state=matching-state&code=bad-code", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "matching-state"})

	resp, err := callbackClient().Do(req)
	if err != nil {
		t.Fatalf("GET /auth/callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}
