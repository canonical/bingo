package auth

import (
	"context"
	"fmt"
	"net/http"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"bingo/internal/config"
)

type contextKey struct{}

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
	return &Provider{
		oidcProvider: oidcProv,
		oauth2Config: oauth2Cfg,
		verifier:     verifier,
		codec:        NewCodec(cfg.SessionSecret),
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
// returns the user's OIDC sub claim and email address.
func (p *Provider) Exchange(ctx context.Context, code string) (sub, email string, err error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return "", "", err
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", "", fmt.Errorf("id_token missing from token response")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", "", err
	}
	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return "", "", err
	}
	return claims.Sub, claims.Email, nil
}

// SetSession encodes the user identity into the session cookie.
func (p *Provider) SetSession(w http.ResponseWriter, userID int64, sub, email string) error {
	return p.codec.Set(w, &Session{UserID: userID, Sub: sub, Email: email})
}

// ClearSession clears the session cookie.
func (p *Provider) ClearSession(w http.ResponseWriter) {
	p.codec.Clear(w)
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
