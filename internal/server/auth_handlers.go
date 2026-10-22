package server

import (
	"log/slog"
	"net/http"

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

	if sessErr := s.auth.SetSession(w, userID, sub, email, idToken); sessErr != nil {
		slog.Error("set session cookie", "err", sessErr)
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

// handleLogout clears the session and CSRF cookies and returns the user home,
// where the SPA shows its sign-in page. Only bingo's own session ends; the
// IdP session survives (as with Mattermost), and unauthenticated users aren't
// redirected to the IdP, so logout doesn't silently sign them back in.
//
// Logout is a POST (it mutates auth state) and requires a valid CSRF token,
// same as other state-changing endpoints, so a third-party site can't force
// a victim's browser to log them out via a bare cross-site GET/link.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !s.requireCSRF(w, r) {
		return
	}
	if s.auth != nil {
		s.auth.ClearSession(w)
	}
	auth.ClearCSRFCookie(w)
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

// validateCallbackState reports whether the oidc_state cookie is present,
// non-empty, and matches the "state" query parameter. This is the CSRF guard
// for the OIDC authorization-code callback.
func validateCallbackState(r *http.Request) bool {
	cookie, err := r.Cookie("oidc_state")
	return err == nil && cookie.Value != "" && cookie.Value == r.URL.Query().Get("state")
}
