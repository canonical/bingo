package server

import (
	"log/slog"
	"net/http"
	"strings"

	"bingo/internal/auth"
)

// handleLogin redirects the user to the OIDC provider's authorization endpoint.
// Returns 403 when auth is not configured.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusForbidden, "auth_disabled", "Authentication is not enabled.")
		return
	}
	state, err := auth.GenerateToken()
	if err != nil {
		slog.Error("generate OIDC state", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return
	}
	// Store state in a short-lived HttpOnly cookie for callback verification.
	// Path is "/" (not scoped to /auth/callback) because reverse proxies that
	// route by path prefix (e.g. Traefik ingress-per-app) strip the prefix
	// before forwarding to the app, but the browser still sees the full
	// prefixed path. A cookie scoped to "/auth/callback" would never be sent
	// back on a request to "/<prefix>/auth/callback".
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.auth.AuthCodeURL(state), http.StatusFound)
}

// handleCallback handles the OIDC authorization code callback.
// Validates the state cookie, exchanges the code, upserts the user, sets
// the session and CSRF cookies, and redirects to /.
func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusForbidden, "auth_disabled", "Authentication is not enabled.")
		return
	}

	// Validate state to prevent CSRF on the callback itself.
	if !validateCallbackState(r) {
		writeError(w, http.StatusBadRequest, "invalid_state", "OIDC state mismatch — possible CSRF.")
		return
	}
	// Clear the state cookie. Path must match the one used when setting it.
	http.SetCookie(w, &http.Cookie{
		Name:   "oidc_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Missing authorization code.")
		return
	}

	sub, email, idToken, err := s.auth.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("OIDC exchange", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Token exchange failed.")
		return
	}

	userID, err := s.userRepo.UpsertUser(r.Context(), sub, email)
	if err != nil {
		slog.Error("upsert user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return
	}

	if err := s.auth.SetSession(w, userID, sub, email, idToken); err != nil {
		slog.Error("set session cookie", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return
	}

	csrfToken, err := auth.GenerateToken()
	if err != nil {
		slog.Error("generate CSRF token", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return
	}
	auth.SetCSRFCookie(w, csrfToken)

	http.Redirect(w, r, s.homeURL(), http.StatusSeeOther)
}

// handleLogout clears the session and CSRF cookies, then ends the user's
// session at the IdP too (RP-initiated logout) before returning them home.
//
// Clearing only bingo's own cookies is not enough: the whole app is gated
// behind auth (see requireAuthMiddleware), so the very next request would
// immediately redirect back to /auth/login — and if the browser still holds
// a valid IdP session (e.g. Kratos), the IdP silently re-authenticates it via
// SSO without prompting, making "logout" appear to do nothing. Redirecting
// through the IdP's end_session_endpoint first ends that IdP session too, so
// the next /auth/login actually shows a login prompt.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var idToken string
	if sess, ok := auth.FromContext(r.Context()); ok {
		idToken = sess.IDToken
	}
	if s.auth != nil {
		s.auth.ClearSession(w)
	}
	auth.ClearCSRFCookie(w)
	if logoutURL := s.auth.LogoutURL(idToken, s.postLogoutRedirectURL()); logoutURL != "" {
		http.Redirect(w, r, logoutURL, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, s.homeURL(), http.StatusSeeOther)
}

// homeURL returns the app's externally-visible root path. Reverse proxies
// that route by path prefix (e.g. Traefik ingress-per-app in its default
// "path" mode) strip that prefix before forwarding the request to the app,
// so a plain "/" redirect would resolve to the proxy's domain root instead
// of the app's externally-visible base path, producing a 404. cfg.BasePath
// (derived from APP_BASE_URL, which paas-charm's go-framework extension
// sets to the externally-visible URL) is the authoritative source for that
// prefix — the same source already used for paste RawURL/URL.
func (s *Server) homeURL() string {
	return s.cfg.BasePath() + "/"
}

// postLogoutRedirectURL returns the full, absolute URL the IdP should send
// the browser back to after RP-initiated logout completes. Unlike homeURL
// (a path, resolved by the browser against the current page), this value is
// sent to a third-party IdP that has no notion of "current page", so it must
// be a complete URL — cfg.BaseURL already carries this (scheme+host+prefix)
// and is the same source used to build the OIDC redirect URL at startup.
func (s *Server) postLogoutRedirectURL() string {
	return strings.TrimSuffix(s.cfg.BaseURL, "/") + "/"
}

// validateCallbackState reports whether the oidc_state cookie is present,
// non-empty, and matches the "state" query parameter. This is the CSRF guard
// for the OIDC authorization-code callback.
func validateCallbackState(r *http.Request) bool {
	cookie, err := r.Cookie("oidc_state")
	return err == nil && cookie.Value != "" && cookie.Value == r.URL.Query().Get("state")
}
