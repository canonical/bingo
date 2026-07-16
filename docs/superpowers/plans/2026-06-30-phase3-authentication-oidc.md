# Phase 3: Authentication (OIDC) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add optional OIDC authentication (CIdP) that enables session-based identity, owner-attributed pastes, and a "my pastes" listing — while keeping the app fully functional anonymously when OIDC is not configured.

**Architecture:** Auth is centred in `internal/auth/`: a `Provider` struct wraps go-oidc + oauth2 and owns the session codec. When the four `OIDC_*` env vars are absent, `Provider` is `nil` and every auth path is a no-op. The session is stored in an AES-GCM encrypted, HttpOnly, SameSite=Strict cookie; no server-side session store is needed. CSRF protection uses the double-submit cookie pattern (JS-readable `csrf_token` cookie + `X-CSRF-Token` request header). CORS restricts allowed origins to `cfg.BaseURL` with a stdlib middleware added to the `ServeHTTP` chain. **Token refresh:** because we store only user identity (not access/refresh tokens) in the session cookie, there is nothing to refresh — re-login re-establishes the cookie. This is intentional for simplicity; the spec mentions refresh in the context of full OAuth session binding which would require server-side storage (out of scope for Phase 3).

**Tech Stack:** `github.com/coreos/go-oidc/v3/oidc` (token validation), `golang.org/x/oauth2` (code exchange), `github.com/gorilla/securecookie` (cookie encryption), `crypto/rand` (CSRF/state tokens), Go 1.22 stdlib.

## Global Constraints

- Module path: `bingo` (not `github.com/canonical/bingo`)
- Go version: 1.22 minimum (go.mod says 1.25.0 — keep that)
- Go binary path in this environment: `/home/daniel.nguyen@canonical.com/go/bin/go`
- Worktree path: `/home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3`
- Run tests with: `export PATH=$PATH:/home/daniel.nguyen@canonical.com/go/bin && go test ./...`
- TDD: write failing test → run to confirm FAIL → implement → run to confirm PASS → commit
- One `TestMain` per test binary — it already lives in `internal/paste/postgres_test.go`
- Integration tests guarded by `DATABASE_URL` env var + `t.Skip()` in `requireDB()`
- Commit co-author trailer on every commit: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`
- Error code vocabulary: `unauthenticated` (401), `forbidden` (403), `csrf_invalid` (403), `auth_disabled` (403)

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/auth/session.go` | Create | `Session` struct, `Codec` (securecookie wrapper), cookie set/read/clear |
| `internal/auth/session_test.go` | Create | Unit tests for cookie encode/decode roundtrip |
| `internal/auth/csrf.go` | Create | CSRF token generation, cookie, and header validation |
| `internal/auth/csrf_test.go` | Create | Unit tests for CSRF validation logic |
| `internal/auth/provider.go` | Create | `Provider` struct: OIDC discovery, code exchange, context injection, auth middleware handler |
| `internal/auth/provider_test.go` | Create | Unit tests for nil-provider no-op behaviour; integration test skipped if OIDC not configured |
| `internal/auth/userrepo.go` | Create | `UserRepository`: upsert user by OIDC `sub` → `users.id` |
| `internal/auth/userrepo_test.go` | Create | Integration tests guarded by `DATABASE_URL` |
| `internal/auth/auth.go` | Modify | Add package doc comment |
| `internal/database/migrations/002_create_users.up.sql` | Create | `users` table + FK from `pastes.owner_id` |
| `internal/database/migrations/002_create_users.down.sql` | Create | Drop FK + `users` table |
| `internal/paste/paste.go` | Modify | Add `ListByOwner` to `Repository` interface |
| `internal/paste/postgres.go` | Modify | Implement `ListByOwner` |
| `internal/paste/postgres_test.go` | Modify | Add `TestPostgresRepository_ListByOwner` integration test |
| `internal/paste/sweep_test.go` | Modify | Update `mockRepository` to implement new `ListByOwner` |
| `internal/server/server.go` | Modify | `New()` accepts `*auth.Provider`; wraps mux with CORS + auth middleware; adds auth + list-my-pastes routes |
| `internal/server/auth_handlers.go` | Create | `handleLogin`, `handleCallback`, `handleLogout` |
| `internal/server/auth_handlers_test.go` | Create | Unit tests using stub provider |
| `internal/server/handlers.go` | Modify | `handleCreatePaste` injects owner; `handleDeletePaste` enforces ownership; add `handleListMyPastes`; CSRF helper |
| `internal/server/server_test.go` | Modify | Update `stubRepo` + `newTestServer`; add ownership/auth tests |
| `cmd/bingo/main.go` | Modify | Initialize `auth.Provider` + pass to `server.New()` |

---

## Task 1: OIDC Dependencies + Config.AuthEnabled()

**Files:**
- Modify: `go.mod` (via `go get`)
- Modify: `internal/config/config.go` — add `AuthEnabled()` + validation
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `cfg.AuthEnabled() bool`, `config.Load()` returns error when partial OIDC config

- [ ] **Step 1: Add dependencies**

```bash
export PATH=$PATH:/home/daniel.nguyen@canonical.com/go/bin
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
go get github.com/coreos/go-oidc/v3/oidc@latest
go get golang.org/x/oauth2@latest
go get github.com/gorilla/securecookie@latest
go mod tidy
```

Expected: `go.mod` now lists `go-oidc/v3`, `oauth2`, `gorilla/securecookie` as direct deps.

- [ ] **Step 2: Write failing tests for AuthEnabled() and config validation**

Add to `internal/config/config_test.go` (after existing tests):

```go
func TestConfig_AuthEnabled_false(t *testing.T) {
    // No OIDC env vars set → auth disabled
    cfg := &config.Config{}
    if cfg.AuthEnabled() {
        t.Error("AuthEnabled() = true, want false when no OIDC vars set")
    }
}

func TestConfig_AuthEnabled_true(t *testing.T) {
    cfg := &config.Config{
        OIDCIssuerURL:    "https://identity.example.com",
        OIDCClientID:     "my-client",
        OIDCClientSecret: "s3cr3t",
        OIDCRedirectURL:  "https://paste.example.com/auth/callback",
        SessionSecret:    "a-long-enough-secret-value-here!",
    }
    if !cfg.AuthEnabled() {
        t.Error("AuthEnabled() = false, want true when all OIDC vars set")
    }
}

func TestLoad_partialOIDCReturnsError(t *testing.T) {
    // Only some OIDC vars set → error
    t.Setenv("OIDC_ISSUER_URL", "https://identity.example.com")
    t.Setenv("OIDC_CLIENT_ID", "my-client")
    // OIDC_CLIENT_SECRET and OIDC_REDIRECT_URL missing
    _, err := config.Load()
    if err == nil {
        t.Error("Load() with partial OIDC config: want error, got nil")
    }
}

func TestLoad_OIDCEnabledRequiresSessionSecret(t *testing.T) {
    t.Setenv("OIDC_ISSUER_URL", "https://identity.example.com")
    t.Setenv("OIDC_CLIENT_ID", "my-client")
    t.Setenv("OIDC_CLIENT_SECRET", "s3cr3t")
    t.Setenv("OIDC_REDIRECT_URL", "https://paste.example.com/auth/callback")
    // SESSION_SECRET not set
    _, err := config.Load()
    if err == nil {
        t.Error("Load() with OIDC enabled but no SESSION_SECRET: want error, got nil")
    }
}
```

Run: `go test ./internal/config/... -v`
Expected: compile error (AuthEnabled undefined).

- [ ] **Step 3: Implement AuthEnabled() and config validation**

Edit `internal/config/config.go` — add method and update `Load()`:

```go
// AuthEnabled reports whether OIDC authentication is fully configured.
// Returns true only when all four OIDC fields and SessionSecret are non-empty.
func (c *Config) AuthEnabled() bool {
    return c.OIDCIssuerURL != "" &&
        c.OIDCClientID != "" &&
        c.OIDCClientSecret != "" &&
        c.OIDCRedirectURL != "" &&
        c.SessionSecret != ""
}
```

Update the `Load()` function body (after reading env vars, before returning):

```go
// Validate OIDC config: either all-or-nothing, and SESSION_SECRET required.
oidcCount := 0
for _, v := range []string{cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.OIDCRedirectURL} {
    if v != "" {
        oidcCount++
    }
}
if oidcCount > 0 && oidcCount < 4 {
    return nil, fmt.Errorf("partial OIDC configuration: all four OIDC_* variables must be set together")
}
if oidcCount == 4 && cfg.SessionSecret == "" {
    return nil, fmt.Errorf("SESSION_SECRET is required when OIDC is configured")
}
```

The full updated `Load()` function:

```go
func Load() (*Config, error) {
    maxSize, err := parseIntEnv("MAX_PASTE_SIZE_BYTES", defaultMaxPasteSizeBytes)
    if err != nil {
        return nil, fmt.Errorf("MAX_PASTE_SIZE_BYTES: %w", err)
    }

    cfg := &Config{
        Port:              envOrDefault("PORT", "8080"),
        DatabaseURL:       os.Getenv("DATABASE_URL"),
        MaxPasteSizeBytes: maxSize,
        BaseURL:           os.Getenv("BASE_URL"),
        LogLevel:          envOrDefault("LOG_LEVEL", "info"),
        OIDCIssuerURL:     os.Getenv("OIDC_ISSUER_URL"),
        OIDCClientID:      os.Getenv("OIDC_CLIENT_ID"),
        OIDCClientSecret:  os.Getenv("OIDC_CLIENT_SECRET"),
        OIDCRedirectURL:   os.Getenv("OIDC_REDIRECT_URL"),
        SessionSecret:     os.Getenv("SESSION_SECRET"),
    }

    oidcCount := 0
    for _, v := range []string{cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.OIDCRedirectURL} {
        if v != "" {
            oidcCount++
        }
    }
    if oidcCount > 0 && oidcCount < 4 {
        return nil, fmt.Errorf("partial OIDC configuration: all four OIDC_* variables must be set together")
    }
    if oidcCount == 4 && cfg.SessionSecret == "" {
        return nil, fmt.Errorf("SESSION_SECRET is required when OIDC is configured")
    }

    return cfg, nil
}
```

- [ ] **Step 4: Run tests**

```bash
export PATH=$PATH:/home/daniel.nguyen@canonical.com/go/bin
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
go test ./internal/config/... -v
```

Expected: All config tests PASS.

Also verify full suite: `go test ./...` — all pass.

- [ ] **Step 5: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
git add go.mod go.sum internal/config/
git commit -m "feat(config): add OIDC deps and AuthEnabled() with validation

- go get go-oidc/v3, oauth2, gorilla/securecookie
- Config.AuthEnabled() returns true only when all 4 OIDC_* + SESSION_SECRET set
- Load() returns error on partial OIDC config or missing SESSION_SECRET

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 2: Session Cookie Codec

**Files:**
- Create: `internal/auth/session.go`
- Create: `internal/auth/session_test.go`

**Interfaces:**
- Consumes: `github.com/gorilla/securecookie`
- Produces:
  - `type Session struct { UserID int64; Sub string; Email string }`
  - `NewCodec(secret string) *Codec`
  - `(*Codec).Set(w http.ResponseWriter, s *Session) error`
  - `(*Codec).Read(r *http.Request) (*Session, bool)`
  - `(*Codec).Clear(w http.ResponseWriter)`

- [ ] **Step 1: Write failing tests**

Create `internal/auth/session_test.go`:

```go
package auth_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "bingo/internal/auth"
)

func TestCodec_roundtrip(t *testing.T) {
    codec := auth.NewCodec("test-secret-key-that-is-long-enough!")

    sess := &auth.Session{UserID: 42, Sub: "sub|abc123", Email: "user@example.com"}

    w := httptest.NewRecorder()
    if err := codec.Set(w, sess); err != nil {
        t.Fatalf("Set() error = %v", err)
    }

    // Build a request with the cookie from the response.
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    for _, c := range w.Result().Cookies() {
        req.AddCookie(c)
    }

    got, ok := codec.Read(req)
    if !ok {
        t.Fatal("Read() returned false, want true")
    }
    if got.UserID != sess.UserID {
        t.Errorf("UserID = %d, want %d", got.UserID, sess.UserID)
    }
    if got.Sub != sess.Sub {
        t.Errorf("Sub = %q, want %q", got.Sub, sess.Sub)
    }
    if got.Email != sess.Email {
        t.Errorf("Email = %q, want %q", got.Email, sess.Email)
    }
}

func TestCodec_Read_noCookie(t *testing.T) {
    codec := auth.NewCodec("test-secret")
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    _, ok := codec.Read(req)
    if ok {
        t.Error("Read() with no cookie = true, want false")
    }
}

func TestCodec_Read_tamperedCookie(t *testing.T) {
    codec := auth.NewCodec("test-secret")
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    req.AddCookie(&http.Cookie{Name: "bingo_session", Value: "tampered!!"})
    _, ok := codec.Read(req)
    if ok {
        t.Error("Read() with tampered cookie = true, want false")
    }
}

func TestCodec_Clear(t *testing.T) {
    codec := auth.NewCodec("test-secret")
    sess := &auth.Session{UserID: 1, Sub: "sub|x", Email: "x@example.com"}

    w := httptest.NewRecorder()
    if err := codec.Set(w, sess); err != nil {
        t.Fatalf("Set(): %v", err)
    }

    // Set then Clear.
    w2 := httptest.NewRecorder()
    codec.Clear(w2)

    // The clear response sets the cookie with MaxAge=-1.
    found := false
    for _, c := range w2.Result().Cookies() {
        if c.Name == "bingo_session" && c.MaxAge < 0 {
            found = true
        }
    }
    if !found {
        t.Error("Clear() did not set bingo_session cookie with MaxAge < 0")
    }
}
```

Run: `go test ./internal/auth/... -v`
Expected: compile error (auth.NewCodec undefined).

- [ ] **Step 2: Implement session.go**

Create `internal/auth/session.go`:

```go
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
```

- [ ] **Step 3: Run tests**

```bash
export PATH=$PATH:/home/daniel.nguyen@canonical.com/go/bin
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
go test ./internal/auth/... -v
```

Expected: All 4 session tests PASS.

Run full suite: `go test ./...` — all pass.

- [ ] **Step 4: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
git add internal/auth/
git commit -m "feat(auth): add Session type and encrypted cookie codec

- Session holds UserID, Sub, email from validated OIDC claims
- Codec wraps gorilla/securecookie with SHA-256 key derivation (AES-256)
- Cookie: HttpOnly, Secure, SameSite=Strict, 7-day MaxAge

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 3: CSRF Helpers + OIDC Provider

**Files:**
- Create: `internal/auth/csrf.go`
- Create: `internal/auth/csrf_test.go`
- Create: `internal/auth/provider.go`
- Create: `internal/auth/provider_test.go`

**Interfaces:**
- Consumes: `Codec` from Task 2; `config.Config`; `go-oidc/v3`; `golang.org/x/oauth2`
- Produces:
  - `GenerateToken() (string, error)` — 32-byte random base64url string
  - `SetCSRFCookie(w, token string)` — non-HttpOnly cookie named `csrf_token`
  - `ClearCSRFCookie(w)`
  - `ValidateCSRF(r *http.Request) bool` — header matches cookie
  - `type Provider struct { ... }`
  - `NewProvider(ctx, cfg *config.Config) (*Provider, error)` — returns nil when auth disabled
  - `(*Provider).AuthCodeURL(state string) string`
  - `(*Provider).Exchange(ctx, code string) (sub, email string, err error)`
  - `(*Provider).SetSession(w, userID int64, sub, email string) error`
  - `(*Provider).Middleware(next http.Handler) http.Handler`
  - `FromContext(ctx context.Context) (*Session, bool)`

- [ ] **Step 1: Write failing tests for CSRF**

Create `internal/auth/csrf_test.go`:

```go
package auth_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "bingo/internal/auth"
)

func TestGenerateToken_length(t *testing.T) {
    tok, err := auth.GenerateToken()
    if err != nil {
        t.Fatalf("GenerateToken() error = %v", err)
    }
    if len(tok) < 32 {
        t.Errorf("token length = %d, want >= 32", len(tok))
    }
}

func TestGenerateToken_unique(t *testing.T) {
    t1, _ := auth.GenerateToken()
    t2, _ := auth.GenerateToken()
    if t1 == t2 {
        t.Error("two tokens are equal — not random enough")
    }
}

func TestValidateCSRF_valid(t *testing.T) {
    tok := "abc123validtoken"
    req := httptest.NewRequest(http.MethodPost, "/", nil)
    req.Header.Set("X-CSRF-Token", tok)
    req.AddCookie(&http.Cookie{Name: "csrf_token", Value: tok})
    if !auth.ValidateCSRF(req) {
        t.Error("ValidateCSRF() = false for matching header and cookie, want true")
    }
}

func TestValidateCSRF_missingHeader(t *testing.T) {
    req := httptest.NewRequest(http.MethodPost, "/", nil)
    req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "tok"})
    if auth.ValidateCSRF(req) {
        t.Error("ValidateCSRF() = true with no header, want false")
    }
}

func TestValidateCSRF_missingCookie(t *testing.T) {
    req := httptest.NewRequest(http.MethodPost, "/", nil)
    req.Header.Set("X-CSRF-Token", "tok")
    if auth.ValidateCSRF(req) {
        t.Error("ValidateCSRF() = true with no cookie, want false")
    }
}

func TestValidateCSRF_mismatch(t *testing.T) {
    req := httptest.NewRequest(http.MethodPost, "/", nil)
    req.Header.Set("X-CSRF-Token", "header-value")
    req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie-value"})
    if auth.ValidateCSRF(req) {
        t.Error("ValidateCSRF() = true with mismatched values, want false")
    }
}

func TestSetCSRFCookie_isNotHttpOnly(t *testing.T) {
    w := httptest.NewRecorder()
    auth.SetCSRFCookie(w, "mytoken")
    resp := w.Result()
    found := false
    for _, c := range resp.Cookies() {
        if c.Name == "csrf_token" {
            found = true
            if c.HttpOnly {
                t.Error("csrf_token cookie must NOT be HttpOnly (JS needs to read it)")
            }
            if c.Value != "mytoken" {
                t.Errorf("csrf_token value = %q, want mytoken", c.Value)
            }
        }
    }
    if !found {
        t.Error("csrf_token cookie not set")
    }
}
```

Run: `go test ./internal/auth/... -v`
Expected: compile error (auth.GenerateToken, ValidateCSRF, SetCSRFCookie undefined).

- [ ] **Step 2: Implement csrf.go**

Create `internal/auth/csrf.go`:

```go
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
```

Run: `go test ./internal/auth/... -v` — all CSRF tests pass.

- [ ] **Step 3: Write failing tests for Provider**

Create `internal/auth/provider_test.go`:

```go
package auth_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

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
```

Run: `go test ./internal/auth/... -v`
Expected: compile error (auth.NewProvider, auth.FromContext undefined).

- [ ] **Step 4: Implement provider.go**

Create `internal/auth/provider.go`:

```go
package auth

import (
    "context"
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
```

Note: the `fmt` package import is needed. Add `"fmt"` to the import block.

- [ ] **Step 5: Run tests**

```bash
export PATH=$PATH:/home/daniel.nguyen@canonical.com/go/bin
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
go test ./internal/auth/... -v
```

Expected: All CSRF tests and Provider tests PASS. (Note: `TestNewProvider_nilWhenAuthDisabled` passes because it hits the `!cfg.AuthEnabled()` early return without making network calls.)

Run full suite: `go test ./...` — all pass.

- [ ] **Step 6: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
git add internal/auth/
git commit -m "feat(auth): add CSRF helpers and OIDC Provider

- csrf.go: GenerateToken (32-byte random), SetCSRFCookie (non-HttpOnly),
  ValidateCSRF (double-submit cookie pattern)
- provider.go: Provider wraps go-oidc/v3; NewProvider returns nil when
  auth disabled; Exchange validates ID token + extracts sub/email;
  Middleware injects *Session into request context; nil-safe throughout

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 4: DB Migration + UserRepository + paste.ListByOwner

**Files:**
- Create: `internal/database/migrations/002_create_users.up.sql`
- Create: `internal/database/migrations/002_create_users.down.sql`
- Create: `internal/auth/userrepo.go`
- Create: `internal/auth/userrepo_test.go`
- Modify: `internal/paste/paste.go` — add `ListByOwner` to `Repository`
- Modify: `internal/paste/postgres.go` — implement `ListByOwner`
- Modify: `internal/paste/postgres_test.go` — add `TestPostgresRepository_ListByOwner`
- Modify: `internal/paste/sweep_test.go` — update `mockRepository`
- Modify: `internal/server/server_test.go` — update `stubRepo`

**Interfaces:**
- Consumes: `*sql.DB` from database package
- Produces:
  - `type UserRepository struct { ... }`
  - `NewUserRepository(db *sql.DB) *UserRepository`
  - `(*UserRepository).UpsertUser(ctx, sub, email string) (int64, error)`
  - `paste.Repository.ListByOwner(ctx context.Context, ownerID int64, limit int) ([]*Paste, error)`

- [ ] **Step 1: Write migration SQL**

Create `internal/database/migrations/002_create_users.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS users (
    id         BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sub        TEXT        NOT NULL UNIQUE,
    email      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE pastes
    ADD CONSTRAINT pastes_owner_id_fkey
    FOREIGN KEY (owner_id) REFERENCES users(id);
```

Create `internal/database/migrations/002_create_users.down.sql`:

```sql
ALTER TABLE pastes DROP CONSTRAINT IF EXISTS pastes_owner_id_fkey;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 2: Write failing tests for ListByOwner and UpsertUser**

Add to `internal/paste/postgres_test.go` (before the final closing brace):

```go
func TestPostgresRepository_ListByOwner(t *testing.T) {
    repo := requireDB(t)
    t.Cleanup(func() { cleanPastes(t) })

    // Insert a fake owner into users.
    var ownerID int64
    err := testDB.QueryRowContext(context.Background(),
        `INSERT INTO users (sub, email) VALUES ($1, $2) RETURNING id`,
        "sub|listtest", "list@example.com",
    ).Scan(&ownerID)
    if err != nil {
        t.Fatalf("insert user: %v", err)
    }
    t.Cleanup(func() {
        testDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", ownerID) //nolint:errcheck
    })

    // Create two pastes for this owner, one for no owner.
    params := paste.CreateParams{
        Content:   "owned paste",
        Language:  "go",
        ExpiresIn: paste.ExpiresIn1d,
        OwnerID:   &ownerID,
    }
    p1, err := repo.Create(context.Background(), params)
    if err != nil {
        t.Fatalf("Create owned: %v", err)
    }
    params.Content = "owned paste 2"
    p2, err := repo.Create(context.Background(), params)
    if err != nil {
        t.Fatalf("Create owned 2: %v", err)
    }
    // Anonymous paste — must NOT appear in results.
    _, err = repo.Create(context.Background(), paste.CreateParams{
        Content:   "anon paste",
        Language:  "go",
        ExpiresIn: paste.ExpiresIn1d,
    })
    if err != nil {
        t.Fatalf("Create anon: %v", err)
    }

    pastes, err := repo.ListByOwner(context.Background(), ownerID, 50)
    if err != nil {
        t.Fatalf("ListByOwner(): %v", err)
    }
    if len(pastes) != 2 {
        t.Fatalf("ListByOwner() returned %d pastes, want 2", len(pastes))
    }
    keys := map[string]bool{p1.Key: true, p2.Key: true}
    for _, p := range pastes {
        if !keys[p.Key] {
            t.Errorf("unexpected key %q in results", p.Key)
        }
    }
}
```

Also create `internal/auth/userrepo_test.go`:

```go
package auth_test

import (
    "context"
    "database/sql"
    "log"
    "os"
    "testing"

    "bingo/internal/auth"
    "bingo/internal/database"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        os.Exit(m.Run())
    }
    var err error
    testDB, err = database.Open(dbURL)
    if err != nil {
        log.Fatalf("open test db: %v", err)
    }
    if err := database.Migrate(testDB); err != nil {
        log.Fatalf("migrate test db: %v", err)
    }
    os.Exit(m.Run())
}

func requireDB(t *testing.T) *auth.UserRepository {
    t.Helper()
    if testDB == nil {
        t.Skip("DATABASE_URL not set — skipping integration test")
    }
    return auth.NewUserRepository(testDB)
}

func TestUserRepository_UpsertUser(t *testing.T) {
    repo := requireDB(t)
    ctx := context.Background()

    sub := "sub|upsert-test-unique"
    email := "upsert@example.com"

    // First upsert — creates.
    id1, err := repo.UpsertUser(ctx, sub, email)
    if err != nil {
        t.Fatalf("UpsertUser() first call error = %v", err)
    }
    if id1 <= 0 {
        t.Errorf("UpsertUser() first call id = %d, want > 0", id1)
    }

    // Second upsert — idempotent, returns same id.
    id2, err := repo.UpsertUser(ctx, sub, email)
    if err != nil {
        t.Fatalf("UpsertUser() second call error = %v", err)
    }
    if id2 != id1 {
        t.Errorf("UpsertUser() second call id = %d, want %d (same)", id2, id1)
    }

    // Cleanup.
    testDB.ExecContext(ctx, "DELETE FROM users WHERE sub = $1", sub) //nolint:errcheck
}
```

Run: `go test ./internal/paste/... ./internal/auth/... -v`
Expected: compile error (paste.Repository.ListByOwner undefined, auth.NewUserRepository undefined).

- [ ] **Step 3: Add ListByOwner to paste.Repository interface**

Edit `internal/paste/paste.go` — add to the `Repository` interface:

```go
// ListByOwner returns up to limit active (non-expired) pastes for ownerID,
// ordered by created_at descending.
ListByOwner(ctx context.Context, ownerID int64, limit int) ([]*Paste, error)
```

Full updated interface:

```go
type Repository interface {
    Create(ctx context.Context, params CreateParams) (*Paste, error)
    GetByKey(ctx context.Context, key string) (*Paste, error)
    Delete(ctx context.Context, key string) error
    DeleteExpired(ctx context.Context) (int64, error)
    ListByOwner(ctx context.Context, ownerID int64, limit int) ([]*Paste, error)
}
```

- [ ] **Step 4: Implement ListByOwner in postgres.go**

Add to `internal/paste/postgres.go`:

```go
// ListByOwner returns up to limit active pastes owned by ownerID, newest first.
func (r *PostgresRepository) ListByOwner(ctx context.Context, ownerID int64, limit int) ([]*Paste, error) {
    const q = `
        SELECT id, key, content, language, title, size_bytes, expires_at, created_at, owner_id
        FROM pastes
        WHERE owner_id = $1 AND expires_at > now()
        ORDER BY created_at DESC
        LIMIT $2`
    rows, err := r.db.QueryContext(ctx, q, ownerID, limit)
    if err != nil {
        return nil, fmt.Errorf("list by owner: %w", err)
    }
    defer rows.Close()

    var pastes []*Paste
    for rows.Next() {
        var p Paste
        var title sql.NullString
        err := rows.Scan(
            &p.ID, &p.Key, &p.Content, &p.Language, &title,
            &p.SizeBytes, &p.ExpiresAt, &p.CreatedAt, &p.OwnerID,
        )
        if err != nil {
            return nil, fmt.Errorf("scan paste: %w", err)
        }
        if title.Valid {
            p.Title = title.String
        }
        pastes = append(pastes, &p)
    }
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("rows error: %w", err)
    }
    return pastes, nil
}
```

- [ ] **Step 5: Update mockRepository in sweep_test.go**

Edit `internal/paste/sweep_test.go` — add `listByOwnerFn` to `mockRepository`:

```go
type mockRepository struct {
    mu                 sync.Mutex
    deleteExpiredCalls int
}

func (m *mockRepository) Create(ctx context.Context, params paste.CreateParams) (*paste.Paste, error) {
    return nil, nil
}

func (m *mockRepository) GetByKey(ctx context.Context, key string) (*paste.Paste, error) {
    return nil, nil
}

func (m *mockRepository) Delete(ctx context.Context, key string) error {
    return nil
}

func (m *mockRepository) DeleteExpired(ctx context.Context) (int64, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.deleteExpiredCalls++
    return 1, nil
}

func (m *mockRepository) ListByOwner(ctx context.Context, ownerID int64, limit int) ([]*paste.Paste, error) {
    return nil, nil
}
```

- [ ] **Step 6: Update stubRepo in server_test.go**

Edit `internal/server/server_test.go` — add `listByOwnerFn` to `stubRepo`:

```go
type stubRepo struct {
    createFn        func(ctx context.Context, p paste.CreateParams) (*paste.Paste, error)
    getByKeyFn      func(ctx context.Context, key string) (*paste.Paste, error)
    deleteFn        func(ctx context.Context, key string) error
    deleteExpiredFn func(ctx context.Context) (int64, error)
    listByOwnerFn   func(ctx context.Context, ownerID int64, limit int) ([]*paste.Paste, error)
}

func (s *stubRepo) Create(ctx context.Context, p paste.CreateParams) (*paste.Paste, error) {
    return s.createFn(ctx, p)
}
func (s *stubRepo) GetByKey(ctx context.Context, key string) (*paste.Paste, error) {
    return s.getByKeyFn(ctx, key)
}
func (s *stubRepo) Delete(ctx context.Context, key string) error {
    return s.deleteFn(ctx, key)
}
func (s *stubRepo) DeleteExpired(ctx context.Context) (int64, error) {
    return s.deleteExpiredFn(ctx)
}
func (s *stubRepo) ListByOwner(ctx context.Context, ownerID int64, limit int) ([]*paste.Paste, error) {
    return s.listByOwnerFn(ctx, ownerID, limit)
}
```

Also update `defaultRepo()` to add the new field:

```go
func defaultRepo() *stubRepo {
    return &stubRepo{
        createFn: func(_ context.Context, _ paste.CreateParams) (*paste.Paste, error) {
            return nil, nil
        },
        getByKeyFn: func(_ context.Context, _ string) (*paste.Paste, error) {
            return nil, paste.ErrNotFound
        },
        deleteFn:        func(_ context.Context, _ string) error { return nil },
        deleteExpiredFn: func(_ context.Context) (int64, error) { return 0, nil },
        listByOwnerFn:   func(_ context.Context, _ int64, _ int) ([]*paste.Paste, error) { return nil, nil },
    }
}
```

- [ ] **Step 7: Implement UserRepository**

Create `internal/auth/userrepo.go`:

```go
package auth

import (
    "context"
    "database/sql"
    "fmt"
)

// UserRepository persists and looks up users by their OIDC sub claim.
type UserRepository struct {
    db *sql.DB
}

// NewUserRepository creates a UserRepository backed by db.
func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}

// UpsertUser ensures a row exists in users for sub, updates email, and returns the id.
// This is idempotent: repeated calls with the same sub return the same id.
func (r *UserRepository) UpsertUser(ctx context.Context, sub, email string) (int64, error) {
    const q = `
        INSERT INTO users (sub, email)
        VALUES ($1, $2)
        ON CONFLICT (sub) DO UPDATE SET email = EXCLUDED.email
        RETURNING id`
    var id int64
    if err := r.db.QueryRowContext(ctx, q, sub, email).Scan(&id); err != nil {
        return 0, fmt.Errorf("upsert user: %w", err)
    }
    return id, nil
}
```

- [ ] **Step 8: Run tests**

```bash
export PATH=$PATH:/home/daniel.nguyen@canonical.com/go/bin
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
go test ./... -v 2>&1 | grep -E "^(ok|FAIL|---)" | head -40
```

Expected:
- `bingo/internal/config` — PASS
- `bingo/internal/key` — PASS
- `bingo/internal/paste` — PASS (integration tests skip without `DATABASE_URL`)
- `bingo/internal/auth` — PASS (UpsertUser integration test skips without `DATABASE_URL`)
- `bingo/internal/server` — PASS

Full suite must pass: `go test ./...`

- [ ] **Step 9: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
git add internal/database/migrations/ internal/auth/userrepo.go internal/auth/userrepo_test.go \
        internal/paste/paste.go internal/paste/postgres.go internal/paste/postgres_test.go \
        internal/paste/sweep_test.go internal/server/server_test.go
git commit -m "feat(auth,paste): add users table, UserRepository, and ListByOwner

- Migration 002: users table (sub UNIQUE, email) + FK pastes.owner_id → users.id
- auth.UserRepository.UpsertUser: INSERT ON CONFLICT DO UPDATE, idempotent
- paste.Repository: add ListByOwner(ctx, ownerID, limit) to interface
- PostgresRepository.ListByOwner: returns active pastes for owner, newest first
- Update all mock/stub repos to implement new interface method

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 5: Auth HTTP Handlers (Login / Callback / Logout)

**Files:**
- Create: `internal/server/auth_handlers.go`
- Create: `internal/server/auth_handlers_test.go`
- Modify: `internal/server/server.go` — add `auth *auth.Provider` field, update `New()`, add routes, wrap mux with middleware

**Interfaces:**
- Consumes: `*auth.Provider` (nil when disabled); `auth.GenerateToken`; `auth.SetCSRFCookie`; `auth.ClearCSRFCookie`; `auth.UserRepository.UpsertUser`
- Produces: routes `GET /auth/login`, `GET /auth/callback`, `GET /auth/logout`; `server.New(cfg, db, repo, authProvider *auth.Provider, userRepo *auth.UserRepository)`

- [ ] **Step 1: Write failing tests**

Create `internal/server/auth_handlers_test.go`:

```go
package server_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "bingo/internal/auth"
    "bingo/internal/config"
    "bingo/internal/server"
)

// newTestServerWithAuth creates a test server with auth wired but disabled (nil provider).
func newTestServerWithAuth(t *testing.T, repo interface {
    // Match the paste.Repository interface that server.New expects.
}, authProvider *auth.Provider) *httptest.Server {
    t.Helper()
    cfg := &config.Config{
        BaseURL:           "https://example.com",
        MaxPasteSizeBytes: 5 * 1024 * 1024,
    }
    srv := server.New(cfg, nil, defaultRepo(), authProvider, nil)
    ts := httptest.NewServer(srv)
    t.Cleanup(ts.Close)
    return ts
}

func TestLogin_authDisabled(t *testing.T) {
    // nil provider → 403 with auth_disabled code
    ts := newTestServerWithAuth(t, defaultRepo(), nil)

    resp, err := http.Get(ts.URL + "/auth/login")
    if err != nil {
        t.Fatalf("GET /auth/login: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusForbidden {
        t.Errorf("status = %d, want 403", resp.StatusCode)
    }
}

func TestLogout_authDisabled(t *testing.T) {
    ts := newTestServerWithAuth(t, defaultRepo(), nil)

    resp, err := http.Get(ts.URL + "/auth/logout")
    if err != nil {
        t.Fatalf("GET /auth/logout: %v", err)
    }
    defer resp.Body.Close()

    // Even when auth disabled, logout clears cookies and redirects to /
    if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
        t.Errorf("logout status = %d, want 302 or 303", resp.StatusCode)
    }
}
```

Note: `newTestServerWithAuth` uses the updated `server.New()` signature — this will fail to compile until Step 2.

Run: `go test ./internal/server/... -v`
Expected: compile error (server.New signature mismatch).

- [ ] **Step 2: Update server.go — new New() signature + auth middleware + routes**

Replace the content of `internal/server/server.go`:

```go
// Package server implements the HTTP server, routing, and request handlers.
package server

import (
    "database/sql"
    "encoding/json"
    "net/http"

    "bingo/internal/auth"
    "bingo/internal/config"
    "bingo/internal/paste"
)

// Server holds the HTTP router, application configuration, and storage.
type Server struct {
    mux      *http.ServeMux
    cfg      *config.Config
    db       *sql.DB
    repo     paste.Repository
    auth     *auth.Provider      // nil when auth is disabled
    userRepo *auth.UserRepository // nil when auth is disabled
}

// New creates a Server with all API routes registered.
// authProvider and userRepo may be nil when authentication is disabled.
// db may be nil in tests that do not exercise the healthz DB ping.
func New(cfg *config.Config, db *sql.DB, repo paste.Repository, authProvider *auth.Provider, userRepo *auth.UserRepository) *Server {
    s := &Server{
        mux:      http.NewServeMux(),
        cfg:      cfg,
        db:       db,
        repo:     repo,
        auth:     authProvider,
        userRepo: userRepo,
    }
    s.registerRoutes()
    return s
}

// ServeHTTP implements http.Handler, delegating through CORS + auth middleware
// and then to the internal ServeMux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    s.corsMiddleware(s.auth.Middleware(s.mux)).ServeHTTP(w, r)
}

// corsMiddleware restricts allowed origins to s.cfg.BaseURL.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        if origin != "" && origin == s.cfg.BaseURL {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Access-Control-Allow-Credentials", "true")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
        }
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// registerRoutes wires all API and auth endpoints.
func (s *Server) registerRoutes() {
    // Auth endpoints
    s.mux.HandleFunc("GET /auth/login", s.handleLogin)
    s.mux.HandleFunc("GET /auth/callback", s.handleCallback)
    s.mux.HandleFunc("GET /auth/logout", s.handleLogout)

    // API endpoints
    s.mux.HandleFunc("GET /api/v1/healthz", s.handleHealthz)
    s.mux.HandleFunc("POST /api/v1/pastes", s.handleCreatePaste)
    s.mux.HandleFunc("GET /api/v1/pastes", s.handleListMyPastes)
    s.mux.HandleFunc("GET /api/v1/pastes/{key}", s.handleGetPaste)
    s.mux.HandleFunc("GET /api/v1/pastes/{key}/raw", s.handleGetPasteRaw)
    s.mux.HandleFunc("DELETE /api/v1/pastes/{key}", s.handleDeletePaste)
    s.mux.HandleFunc("GET /api/v1/languages", s.handleListLanguages)
}

// writeJSON sets Content-Type: application/json, writes the status, and encodes v.
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(v)
}

// errDetail is the inner object in all error responses.
type errDetail struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

// errEnvelope is the standard JSON error body.
type errEnvelope struct {
    Error errDetail `json:"error"`
}

// writeError writes a JSON error envelope with the given HTTP status.
func writeError(w http.ResponseWriter, status int, code, message string) {
    writeJSON(w, status, errEnvelope{Error: errDetail{Code: code, Message: message}})
}
```

- [ ] **Step 3: Create auth_handlers.go**

Create `internal/server/auth_handlers.go`:

```go
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
        SameSite: http.SameSiteStrictMode,
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
    stateCookie, err := r.Cookie("oidc_state")
    if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
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
```

- [ ] **Step 4: Fix all callers of server.New() in tests and main.go**

In `internal/server/server_test.go`, update `newTestServer()`:

```go
func newTestServer(t *testing.T, repo paste.Repository) *httptest.Server {
    t.Helper()
    cfg := &config.Config{
        BaseURL:           "https://example.com",
        MaxPasteSizeBytes: 5 * 1024 * 1024,
    }
    srv := server.New(cfg, nil, repo, nil, nil) // nil auth: disabled
    ts := httptest.NewServer(srv)
    t.Cleanup(ts.Close)
    return ts
}
```

Also fix the `TestCreatePaste_tooLarge` test which creates its own server:

```go
func TestCreatePaste_tooLarge(t *testing.T) {
    cfg := &config.Config{
        BaseURL:           "https://example.com",
        MaxPasteSizeBytes: 10,
    }
    srv := server.New(cfg, nil, defaultRepo(), nil, nil)
    ts := httptest.NewServer(srv)
    t.Cleanup(ts.Close)
    // ... rest of test unchanged
}
```

And `TestCreatePaste_atSizeLimit`:

```go
func TestCreatePaste_atSizeLimit(t *testing.T) {
    const limit = 10
    cfg := &config.Config{
        BaseURL:           "https://example.com",
        MaxPasteSizeBytes: limit,
    }
    // ... repo setup unchanged ...
    srv := server.New(cfg, nil, repo, nil, nil)
    ts := httptest.NewServer(srv)
    t.Cleanup(ts.Close)
    // ... rest unchanged
}
```

Update `cmd/bingo/main.go` — temporarily pass nil for authProvider (will be wired in T6):

```go
srv := server.New(cfg, db, repo, nil, nil)
```

- [ ] **Step 5: Run tests**

```bash
export PATH=$PATH:/home/daniel.nguyen@canonical.com/go/bin
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
go test ./... -v 2>&1 | grep -E "^(ok|FAIL|---)" | head -40
```

Expected: all packages PASS. The auth handler tests (`TestLogin_authDisabled`, `TestLogout_authDisabled`) pass because `s.auth == nil` → 403 / redirect.

- [ ] **Step 6: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
git add internal/server/ cmd/bingo/main.go
git commit -m "feat(server): add auth endpoints and update New() signature

- server.New() now accepts *auth.Provider and *auth.UserRepository (nil = disabled)
- ServeHTTP wraps mux with auth.Middleware (session injection, nil-safe)
- GET /auth/login: generate state, set oidc_state cookie, redirect to provider
- GET /auth/callback: validate state, exchange code, upsert user, set session+CSRF cookies
- GET /auth/logout: clear session+CSRF cookies, redirect to /
- When auth disabled: login → 403; logout → clears cookies + redirect

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 6: Owner Enforcement + My-Pastes Endpoint + main.go Wiring

**Files:**
- Modify: `internal/server/handlers.go` — update `handleCreatePaste` (inject ownerID), `handleDeletePaste` (enforce ownership), add `handleListMyPastes`, add `requireCSRF` helper
- Modify: `internal/server/server_test.go` — add ownership tests, CSRF tests, my-pastes tests
- Modify: `cmd/bingo/main.go` — initialise `auth.Provider` + `auth.UserRepository`, pass to `server.New()`

**Interfaces:**
- Consumes: `auth.FromContext`, `auth.ValidateCSRF`, `paste.Repository.ListByOwner`
- Produces: enforced ownership on DELETE; ownerID from context on POST; `GET /api/v1/pastes?mine=true`

- [ ] **Step 1: Write failing tests for ownership enforcement and my-pastes**

Add to `internal/server/server_test.go`:

```go
func TestDeletePaste_anonymousPasteForbidden(t *testing.T) {
    repo := defaultRepo()
    repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
        return &paste.Paste{
            Key:      "anon",
            OwnerID:  nil, // anonymous paste
            ExpiresAt: time.Now().Add(time.Hour),
        }, nil
    }
    ts := newTestServer(t, repo)

    req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/pastes/anon", nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("DELETE: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusForbidden {
        t.Errorf("delete anonymous paste: status = %d, want 403", resp.StatusCode)
    }
}

func TestDeletePaste_unauthenticatedOnOwnedPaste(t *testing.T) {
    ownerID := int64(1)
    repo := defaultRepo()
    repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
        return &paste.Paste{
            Key:       "owned",
            OwnerID:   &ownerID,
            ExpiresAt: time.Now().Add(time.Hour),
        }, nil
    }
    ts := newTestServer(t, repo) // no auth → no session in context

    req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/pastes/owned", nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("DELETE: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusUnauthorized {
        t.Errorf("unauthenticated delete of owned paste: status = %d, want 401", resp.StatusCode)
    }
}

func TestListMyPastes_authDisabled(t *testing.T) {
    ts := newTestServer(t, defaultRepo())

    resp, err := http.Get(ts.URL + "/api/v1/pastes?mine=true")
    if err != nil {
        t.Fatalf("GET: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusUnauthorized {
        t.Errorf("status = %d, want 401", resp.StatusCode)
    }
}

func TestListMyPastes_badQueryParam(t *testing.T) {
    ts := newTestServer(t, defaultRepo())

    resp, err := http.Get(ts.URL + "/api/v1/pastes")
    if err != nil {
        t.Fatalf("GET: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusBadRequest {
        t.Errorf("status = %d, want 400", resp.StatusCode)
    }
}

func TestCreatePaste_injectsOwnerIDFromSession(t *testing.T) {
    var capturedOwnerID *int64
    repo := defaultRepo()
    repo.createFn = func(_ context.Context, p paste.CreateParams) (*paste.Paste, error) {
        capturedOwnerID = p.OwnerID
        return &paste.Paste{
            Key:       "xyz",
            Content:   p.Content,
            Language:  p.Language,
            SizeBytes: len(p.Content),
            ExpiresAt: time.Now().Add(time.Hour),
            CreatedAt: time.Now(),
        }, nil
    }
    // Build a handler that injects a session into context before calling server.
    // We do this by wrapping the test server with a middleware that sets the context.
    cfg := &config.Config{BaseURL: "https://example.com", MaxPasteSizeBytes: 5 * 1024 * 1024}
    srv := server.New(cfg, nil, repo, nil, nil)
    wrapped := injectSession(srv, &auth.Session{UserID: 99, Sub: "sub|99", Email: "x@x.com"})
    ts := httptest.NewServer(wrapped)
    t.Cleanup(ts.Close)

    body := `{"content":"hello","language":"go","expires_in":"1d"}`
    resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json", strings.NewReader(body))
    if err != nil {
        t.Fatalf("POST: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        t.Errorf("status = %d, want 201", resp.StatusCode)
    }
    if capturedOwnerID == nil || *capturedOwnerID != 99 {
        t.Errorf("OwnerID = %v, want pointer to 99", capturedOwnerID)
    }
}

// injectSession wraps handler with middleware that forces a specific session into context.
func injectSession(next http.Handler, sess *auth.Session) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := context.WithValue(r.Context(), auth.TestSessionKey{}, sess)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Important:** The `injectSession` test helper above requires that `auth.FromContext` can also read from a test context key. The cleanest approach is to export the context key type for testing. Add to `internal/auth/provider.go`:

```go
// TestSessionKey is the exported context key for use in tests that need to
// inject a session without a real session cookie. Production code uses
// the unexported contextKey{}.
type TestSessionKey = contextKey
```

And update `FromContext` to use `contextKey{}`:

```go
func FromContext(ctx context.Context) (*Session, bool) {
    s, ok := ctx.Value(contextKey{}).(*Session)
    return s, ok && s != nil
}
```

The `injectSession` wrapper uses `contextKey{}` (via the alias) to inject into the same slot that `Middleware` uses. This makes the test valid.

Also add `"context"` and `"strings"` imports to `server_test.go` if not already present, and `"bingo/internal/auth"`.

Run: `go test ./internal/server/... -v`
Expected: compile errors (handleListMyPastes undefined, handleDeletePaste doesn't enforce ownership).

- [ ] **Step 2: Implement ownership enforcement and my-pastes in handlers.go**

Update `internal/server/handlers.go`:

**a) Add `requireCSRF` helper** (add after the existing helpers):

```go
// requireCSRF validates the CSRF token for authenticated requests.
// Returns true if the request should proceed. When auth is disabled or the
// user is anonymous, CSRF is not required.
func (s *Server) requireCSRF(w http.ResponseWriter, r *http.Request) bool {
    if s.auth == nil {
        return true
    }
    _, authenticated := auth.FromContext(r.Context())
    if !authenticated {
        return true
    }
    if !auth.ValidateCSRF(r) {
        writeError(w, http.StatusForbidden, "csrf_invalid", "CSRF token missing or invalid.")
        return false
    }
    return true
}
```

Add `"bingo/internal/auth"` to imports.

**b) Update `handleCreatePaste`** — extract ownerID from context and pass to repo:

After the `params` declaration and before the `s.repo.Create` call, add:

```go
// Inject ownerID from session when the user is authenticated.
if sess, ok := auth.FromContext(r.Context()); ok {
    id := sess.UserID
    params.OwnerID = &id
}
```

So the updated block looks like:

```go
params := paste.CreateParams{
    Content:   req.Content,
    Language:  req.Language,
    Title:     req.Title,
    ExpiresIn: ei,
}
if sess, ok := auth.FromContext(r.Context()); ok {
    id := sess.UserID
    params.OwnerID = &id
}
p, err := s.repo.Create(r.Context(), params)
```

**c) Replace `handleDeletePaste`** — add ownership enforcement:

```go
func (s *Server) handleDeletePaste(w http.ResponseWriter, r *http.Request) {
    k := r.PathValue("key")

    // Fetch paste to check ownership before deleting.
    p, err := s.repo.GetByKey(r.Context(), k)
    if err != nil {
        if errors.Is(err, paste.ErrNotFound) {
            writeError(w, http.StatusNotFound, "paste_not_found", "Paste not found.")
            return
        }
        slog.Error("get paste for delete", "err", err)
        writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
        return
    }

    if p.OwnerID == nil {
        // Anonymous pastes can only be removed by expiry, not explicitly deleted.
        writeError(w, http.StatusForbidden, "forbidden", "Anonymous pastes cannot be deleted.")
        return
    }

    sess, ok := auth.FromContext(r.Context())
    if !ok {
        writeError(w, http.StatusUnauthorized, "unauthenticated", "Login required to delete this paste.")
        return
    }
    if sess.UserID != *p.OwnerID {
        writeError(w, http.StatusForbidden, "forbidden", "You do not own this paste.")
        return
    }

    if err := s.repo.Delete(r.Context(), k); err != nil {
        slog.Error("delete paste", "err", err)
        writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

**d) Add `handleListMyPastes`**:

```go
// pasteListItem is a paste summary without the (potentially large) content field.
type pasteListItem struct {
    Key       string    `json:"key"`
    URL       string    `json:"url"`
    Language  string    `json:"language"`
    Title     string    `json:"title,omitempty"`
    SizeBytes int       `json:"size_bytes"`
    ExpiresAt time.Time `json:"expires_at"`
    CreatedAt time.Time `json:"created_at"`
}

func (s *Server) handleListMyPastes(w http.ResponseWriter, r *http.Request) {
    if r.URL.Query().Get("mine") != "true" {
        writeError(w, http.StatusBadRequest, "invalid_request",
            "This endpoint requires ?mine=true.")
        return
    }

    sess, ok := auth.FromContext(r.Context())
    if !ok {
        writeError(w, http.StatusUnauthorized, "unauthenticated", "Login required.")
        return
    }

    pastes, err := s.repo.ListByOwner(r.Context(), sess.UserID, 50)
    if err != nil {
        slog.Error("list my pastes", "err", err)
        writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
        return
    }

    items := make([]pasteListItem, len(pastes))
    for i, p := range pastes {
        items[i] = pasteListItem{
            Key:       p.Key,
            URL:       s.cfg.BaseURL + "/" + p.Key,
            Language:  p.Language,
            Title:     p.Title,
            SizeBytes: p.SizeBytes,
            ExpiresAt: p.ExpiresAt.UTC(),
            CreatedAt: p.CreatedAt.UTC(),
        }
    }
    writeJSON(w, http.StatusOK, map[string]any{
        "pastes": items,
        "count":  len(items),
    })
}
```

- [ ] **Step 3: Update main.go to wire auth.Provider**

Replace the relevant section of `cmd/bingo/main.go`:

```go
import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "bingo/internal/auth"
    "bingo/internal/config"
    "bingo/internal/database"
    "bingo/internal/paste"
    "bingo/internal/server"
)

func run() error {
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("load config: %w", err)
    }

    logger := newLogger(cfg.LogLevel)
    slog.SetDefault(logger)

    db, err := database.Open(cfg.DatabaseURL)
    if err != nil {
        return fmt.Errorf("open database: %w", err)
    }
    defer db.Close()

    if err := database.Migrate(db); err != nil {
        return fmt.Errorf("migrate database: %w", err)
    }
    slog.Info("database migrations applied")

    repo := paste.NewPostgresRepository(db)

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    cancelSweep := paste.StartSweep(ctx, repo, time.Hour)
    defer cancelSweep()

    authProvider, err := auth.NewProvider(ctx, cfg)
    if err != nil {
        return fmt.Errorf("init OIDC provider: %w", err)
    }
    if authProvider != nil {
        slog.Info("OIDC authentication enabled", "issuer", cfg.OIDCIssuerURL)
    } else {
        slog.Info("OIDC not configured — running in anonymous mode")
    }

    var userRepo *auth.UserRepository
    if authProvider != nil {
        userRepo = auth.NewUserRepository(db)
    }

    srv := server.New(cfg, db, repo, authProvider, userRepo)

    httpSrv := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      srv,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    go func() {
        slog.Info("server starting", "addr", httpSrv.Addr)
        if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("server error", "err", err)
            stop()
        }
    }()

    <-ctx.Done()
    slog.Info("shutting down")

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    return httpSrv.Shutdown(shutdownCtx)
}
```

- [ ] **Step 4: Run full test suite**

```bash
export PATH=$PATH:/home/daniel.nguyen@canonical.com/go/bin
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
go test ./... -count=1 2>&1
```

Expected output:
```
ok   bingo/internal/config
ok   bingo/internal/key
ok   bingo/internal/paste
ok   bingo/internal/auth
ok   bingo/internal/server
```

Also verify build: `go build ./...`

- [ ] **Step 5: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase3
git add internal/server/handlers.go internal/server/server_test.go \
        internal/server/auth_handlers.go cmd/bingo/main.go
git commit -m "feat(server,main): enforce ownership, add CSRF, wire auth provider

- handleCreatePaste: injects session.UserID as OwnerID when authenticated
- handleDeletePaste: anonymous paste → 403; owned paste without session → 401;
  wrong owner → 403; correct owner → 204
- handleListMyPastes: GET /api/v1/pastes?mine=true; requires active session;
  returns [{key,url,language,title,size_bytes,expires_at,created_at},...] + count
- requireCSRF: validates X-CSRF-Token header vs csrf_token cookie for
  authenticated state-changing requests
- main.go: auth.NewProvider on startup; anonymous mode log when disabled

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Self-Review Checklist

| Spec Requirement | Task |
|-----------------|------|
| OIDC optional — app starts without OIDC vars | T1 (AuthEnabled), T3 (nil Provider), T6 (main anonymous mode log) |
| All four OIDC_* vars required together | T1 (partial OIDC error) |
| SESSION_SECRET required when OIDC enabled | T1 |
| Token validation (signature, issuer, audience, expiry) | T3 (go-oidc `Verifier`) |
| Secure HttpOnly SameSite=Strict session cookie | T2 |
| CSRF protection on state-changing requests | T3 (csrf.go) + T6 (requireCSRF) |
| Anonymous paste owner_id NULL | T4 (CreateParams.OwnerID nil) |
| Authenticated paste owner_id from session | T6 (handleCreatePaste) |
| Only owner can delete owned paste | T6 (handleDeletePaste) |
| Anonymous pastes removed only by expiry | T6 (403 on delete) |
| GET /api/v1/pastes?mine=true (auth required) | T6 (handleListMyPastes) |
| users table + FK | T4 (migration 002) |
| OIDC authorization code flow | T3 (AuthCodeURL), T5 (handleLogin/Callback) |
| State cookie CSRF protection on callback | T5 (oidc_state cookie) |
| CSRF cookies set/cleared on login/logout | T5 (handleCallback/Logout) |
| Upsert user on callback | T5 + T4 (UpsertUser) |
| 401 `unauthenticated` code | T6 |
| 403 `forbidden` code | T6 |
| CORS restricted to BaseURL origin (no wildcard) | T5 (corsMiddleware in server.go) |
| Token refresh — not implemented (identity-only cookie, no token persistence) | Architecture note — deferred |
