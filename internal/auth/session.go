package auth

import (
	"crypto/sha256"
	"net/http"

	"github.com/gorilla/securecookie"
)

const sessionCookieName = "bingo_session"

// Session holds authenticated user identity decoded from the session cookie.
type Session struct {
	UserID int64  `json:"user_id"`
	Sub    string `json:"sub"`
	Email  string `json:"email"`
	// IDToken is the raw OIDC ID token issued at login. It is retained only
	// so RP-initiated logout (see Provider.LogoutURL) can pass it back to
	// the IdP as an id_token_hint; it is never used for authorization.
	IDToken string `json:"id_token,omitempty"`
}

// Codec encodes and decodes Session values into encrypted, signed cookies.
type Codec struct {
	sc *securecookie.SecureCookie
}

// NewCodec creates a Codec keyed from secret.
// hashKey = SHA-256(secret), blockKey = SHA-256("block:"+secret) → AES-256.
func NewCodec(secret string) *Codec {
	h := sha256.Sum256([]byte(secret))
	b := sha256.Sum256(append([]byte("block:"), []byte(secret)...))
	return &Codec{sc: securecookie.New(h[:], b[:])}
}

// Set encodes sess and writes the session cookie to w.
//
// Secure is always true, so browsers only send this cookie over HTTPS. This
// is required to protect the session from interception, but it means the
// cookie is silently dropped (no error) when the app is served over plain
// HTTP, e.g. a local dev server at http://localhost. In that case OIDC login
// will appear to complete but no session will persist, producing an
// unexplained redirect-to-login loop. Serve over HTTPS (a local reverse
// proxy with a self-signed/mkcert certificate) to test auth locally.
func (c *Codec) Set(w http.ResponseWriter, sess *Session) error {
	encoded, err := c.sc.Encode(sessionCookieName, sess)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

// Read decodes the session cookie from r. Returns false when absent or invalid.
func (c *Codec) Read(r *http.Request) (*Session, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, false
	}
	var sess Session
	if err := c.sc.Decode(sessionCookieName, cookie.Value, &sess); err != nil {
		return nil, false
	}
	return &sess, true
}

// Clear writes an expired session cookie to w, deleting it in the browser.
func (c *Codec) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}
