package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"bingo/internal/config"
)

type contextKey struct{}

// TestSessionKey is an exported alias for the session context key.
// Use it in tests to inject a *Session without a real session cookie:
//
//	ctx := context.WithValue(r.Context(), auth.TestSessionKey{}, &auth.Session{...})
type TestSessionKey = contextKey

// FromContext returns the authenticated Session from ctx, if any.
func FromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(contextKey{}).(*Session)
	return s, ok && s != nil
}

// Provider wraps the go-oidc provider and oauth2 config for the OIDC auth flow.
// A nil *Provider is safe to use — all methods become no-ops.
type Provider struct {
	oidcProvider *gooidc.Provider
	oauth2Config oauth2.Config
	verifier     *gooidc.IDTokenVerifier
	codec        *Codec
	// endSessionEndpoint is the IdP's RP-initiated logout endpoint (OIDC
	// "end session" endpoint), read from discovery metadata if advertised.
	// Empty when the IdP doesn't support it — LogoutURL then returns "".
	endSessionEndpoint string
}

// NewProvider initialises the OIDC provider by fetching discovery metadata from
// cfg.OIDCIssuerURL. Returns (nil, nil) when auth is not configured.
func NewProvider(ctx context.Context, cfg *config.Config) (*Provider, error) {
	if !cfg.AuthEnabled() {
		return nil, nil
	}
	oidcProv, err := gooidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, err
	}
	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		Endpoint:     oidcProv.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
	}
	verifier := oidcProv.Verifier(&gooidc.Config{ClientID: cfg.OIDCClientID})
	var discovery struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	_ = oidcProv.Claims(&discovery) // best-effort; not all IdPs advertise this
	return &Provider{
		oidcProvider:       oidcProv,
		oauth2Config:       oauth2Cfg,
		verifier:           verifier,
		codec:              NewCodec(cfg.SessionSecret),
		endSessionEndpoint: discovery.EndSessionEndpoint,
	}, nil
}

// AuthCodeURL returns the OIDC authorization redirect URL with the given state.
func (p *Provider) AuthCodeURL(state string) string {
	if p == nil {
		return ""
	}
	return p.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange exchanges the authorization code for an ID token, validates it, and
// returns the user's OIDC sub claim, email address, and the raw ID token
// (retained for RP-initiated logout; see Session.IDToken and LogoutURL).
func (p *Provider) Exchange(ctx context.Context, code string) (sub, email, rawIDToken string, err error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return "", "", "", err
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", "", "", fmt.Errorf("id_token missing from token response")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", "", "", err
	}
	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return "", "", "", err
	}
	return claims.Sub, claims.Email, rawIDToken, nil
}

// SetSession encodes the user identity into the session cookie. idToken is
// the raw ID token from Exchange, retained solely so a later LogoutURL call
// can pass it back to the IdP as an id_token_hint.
func (p *Provider) SetSession(w http.ResponseWriter, userID int64, sub, email, idToken string) error {
	return p.codec.Set(w, &Session{UserID: userID, Sub: sub, Email: email, IDToken: idToken})
}

// ClearSession clears the session cookie.
func (p *Provider) ClearSession(w http.ResponseWriter) {
	p.codec.Clear(w)
}

// LogoutURL returns the IdP's RP-initiated logout URL for idTokenHint,
// which will redirect the browser back to postLogoutRedirectURI once the
// IdP's own session (e.g. Kratos) has been ended. Returns "" when auth is
// disabled or the IdP doesn't advertise an end_session_endpoint — callers
// should fall back to a plain local redirect in that case.
//
// Without this, clearing only bingo's own session cookie is insufficient:
// since the whole app is gated behind auth, the very next request
// immediately redirects to /auth/login, and if the browser still holds a
// valid IdP session, the IdP silently re-authenticates it (SSO) — the user
// never appears logged out.
func (p *Provider) LogoutURL(idTokenHint, postLogoutRedirectURI string) string {
	if p == nil || p.endSessionEndpoint == "" {
		return ""
	}
	v := url.Values{}
	if idTokenHint != "" {
		v.Set("id_token_hint", idTokenHint)
	}
	v.Set("post_logout_redirect_uri", postLogoutRedirectURI)
	v.Set("client_id", p.oauth2Config.ClientID)
	return p.endSessionEndpoint + "?" + v.Encode()
}

// Middleware returns an http.Handler that reads the session cookie and injects a
// *Session into the request context when valid. Safe to call on a nil *Provider.
func (p *Provider) Middleware(next http.Handler) http.Handler {
	if p == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sess, ok := p.codec.Read(r); ok {
			r = r.WithContext(context.WithValue(r.Context(), contextKey{}, sess))
		}
		next.ServeHTTP(w, r)
	})
}
