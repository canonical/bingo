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
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/auth/callback",
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
	// Clear the state cookie.
	http.SetCookie(w, &http.Cookie{
		Name:   "oidc_state",
		Value:  "",
		Path:   "/auth/callback",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Missing authorization code.")
		return
	}

	sub, email, err := s.auth.Exchange(r.Context(), code)
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

	if err := s.auth.SetSession(w, userID, sub, email); err != nil {
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

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogout clears the session and CSRF cookies, then redirects to /.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.auth != nil {
		s.auth.ClearSession(w)
	}
	auth.ClearCSRFCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// validateCallbackState reports whether the oidc_state cookie is present,
// non-empty, and matches the "state" query parameter. This is the CSRF guard
// for the OIDC authorization-code callback.
func validateCallbackState(r *http.Request) bool {
	cookie, err := r.Cookie("oidc_state")
	return err == nil && cookie.Value != "" && cookie.Value == r.URL.Query().Get("state")
}
