package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

const csrfCookieName = "csrf_token"
const csrfHeaderName = "X-CSRF-Token"

// GenerateToken returns a cryptographically random 32-byte base64url string.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// SetCSRFCookie writes a non-HttpOnly csrf_token cookie to w.
// The cookie is deliberately NOT HttpOnly so the frontend JS can read it and
// include it in the X-CSRF-Token request header.
func SetCSRFCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
		HttpOnly: false, // JS must read this
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearCSRFCookie writes an expired csrf_token cookie.
func ClearCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ValidateCSRF returns true when the X-CSRF-Token header matches the csrf_token cookie.
func ValidateCSRF(r *http.Request) bool {
	header := r.Header.Get(csrfHeaderName)
	if header == "" {
		return false
	}
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return false
	}
	return header == cookie.Value
}
