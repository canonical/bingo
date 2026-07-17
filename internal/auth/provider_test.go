package auth_test

import (
	"context"
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

// fakeIdPClaims controls the identity claims embedded in the fake IdP's
// signed ID token.
type fakeIdPClaims struct {
	Sub   string
	Email string
}

// newFakeIdPServer starts a fake OIDC IdP exposing discovery, JWKS, and a
// token endpoint, so tests can exercise Provider.Exchange's full
// exchange-and-verify flow without a real IdP.
//
// The token endpoint's behaviour is selected by the authorization "code"
// presented to it:
//   - "bad-code": returns an HTTP 400 (simulates a rejected exchange).
//   - "no-id-token": returns a token response with no id_token field.
//   - anything else: returns a well-formed token response with a real
//     RS256-signed ID token asserting claims.
func newFakeIdPServer(t *testing.T, clientID string, claims fakeIdPClaims) *httptest.Server {
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
		code := r.FormValue("code")
		if code == "bad-code" {
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
			Subject:  claims.Sub,
			Audience: jwt.Audience{clientID},
			Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		}
		emailClaims := struct {
			Email string `json:"email"`
		}{Email: claims.Email}
		rawIDToken, err := jwt.Signed(signer).Claims(stdClaims).Claims(emailClaims).Serialize()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		if code != "no-id-token" {
			resp["id_token"] = rawIDToken
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	issuer = ts.URL
	return ts
}

func newTestProvider(t *testing.T, idp *httptest.Server, clientID string) *auth.Provider {
	t.Helper()
	cfg := &config.Config{
		OIDCIssuerURL:    idp.URL,
		OIDCClientID:     clientID,
		OIDCClientSecret: "test-secret",
		OIDCRedirectURL:  "https://app.example.com/auth/callback",
		SessionSecret:    "test-session-secret",
	}
	p, err := auth.NewProvider(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if p == nil {
		t.Fatal("NewProvider() = nil, want non-nil")
	}
	return p
}

func TestProvider_Exchange_success(t *testing.T) {
	idp := newFakeIdPServer(t, "test-client", fakeIdPClaims{Sub: "sub|123", Email: "user@example.com"})
	p := newTestProvider(t, idp, "test-client")

	sub, email, rawIDToken, err := p.Exchange(t.Context(), "good-code")
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if sub != "sub|123" {
		t.Errorf("sub = %q, want %q", sub, "sub|123")
	}
	if email != "user@example.com" {
		t.Errorf("email = %q, want %q", email, "user@example.com")
	}
	if rawIDToken == "" {
		t.Error("rawIDToken is empty, want a signed JWT")
	}
}

func TestProvider_Exchange_tokenEndpointError(t *testing.T) {
	idp := newFakeIdPServer(t, "test-client", fakeIdPClaims{Sub: "sub|123", Email: "user@example.com"})
	p := newTestProvider(t, idp, "test-client")

	_, _, _, err := p.Exchange(t.Context(), "bad-code")
	if err == nil {
		t.Fatal("Exchange() with rejected code: want error, got nil")
	}
}

func TestProvider_Exchange_missingIDToken(t *testing.T) {
	idp := newFakeIdPServer(t, "test-client", fakeIdPClaims{Sub: "sub|123", Email: "user@example.com"})
	p := newTestProvider(t, idp, "test-client")

	_, _, _, err := p.Exchange(t.Context(), "no-id-token")
	if err == nil {
		t.Fatal("Exchange() with no id_token in response: want error, got nil")
	}
}

func TestProvider_AuthCodeURL(t *testing.T) {
	idp := newFakeIdPServer(t, "test-client", fakeIdPClaims{})
	p := newTestProvider(t, idp, "test-client")

	got := p.AuthCodeURL("the-state")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("AuthCodeURL() returned unparsable URL %q: %v", got, err)
	}
	q := u.Query()
	if q.Get("state") != "the-state" {
		t.Errorf("state = %q, want %q", q.Get("state"), "the-state")
	}
	if q.Get("client_id") != "test-client" {
		t.Errorf("client_id = %q, want %q", q.Get("client_id"), "test-client")
	}
	if q.Get("redirect_uri") != "https://app.example.com/auth/callback" {
		t.Errorf("redirect_uri = %q, want the configured redirect URL", q.Get("redirect_uri"))
	}
}

func TestProvider_AuthCodeURL_nilProviderReturnsEmpty(t *testing.T) {
	var p *auth.Provider
	if got := p.AuthCodeURL("state"); got != "" {
		t.Errorf("AuthCodeURL() on nil provider = %q, want empty", got)
	}
}

func TestProvider_SetSession_and_Middleware(t *testing.T) {
	idp := newFakeIdPServer(t, "test-client", fakeIdPClaims{})
	p := newTestProvider(t, idp, "test-client")

	rec := httptest.NewRecorder()
	if err := p.SetSession(rec, 42, "sub|abc", "user@example.com", "id-token-value"); err != nil {
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

	// Feed the cookie through Middleware and confirm the session is injected.
	var gotSession *auth.Session
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := auth.FromContext(r.Context())
		if ok {
			gotSession = sess
		}
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(sessionCookie)
	p.Middleware(next).ServeHTTP(httptest.NewRecorder(), req)

	if gotSession == nil {
		t.Fatal("Middleware() did not inject a session from the cookie set by SetSession()")
	}
	if gotSession.UserID != 42 || gotSession.Sub != "sub|abc" || gotSession.Email != "user@example.com" || gotSession.IDToken != "id-token-value" {
		t.Errorf("Middleware() injected session = %+v, want UserID=42 Sub=sub|abc Email=user@example.com IDToken=id-token-value", gotSession)
	}
}

func TestProvider_Middleware_noCookieNoSession(t *testing.T) {
	idp := newFakeIdPServer(t, "test-client", fakeIdPClaims{})
	p := newTestProvider(t, idp, "test-client")

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if _, ok := auth.FromContext(r.Context()); ok {
			t.Error("Middleware() injected a session with no cookie present")
		}
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	p.Middleware(next).ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Error("Middleware() did not call next handler")
	}
}

func TestProvider_ClearSession(t *testing.T) {
	idp := newFakeIdPServer(t, "test-client", fakeIdPClaims{})
	p := newTestProvider(t, idp, "test-client")

	rec := httptest.NewRecorder()
	p.ClearSession(rec)

	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "bingo_session" && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("ClearSession() did not clear the bingo_session cookie")
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
