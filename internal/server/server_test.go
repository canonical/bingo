package server_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" driver for healthz DB test

	"bingo/internal/auth"
	"bingo/internal/config"
	"bingo/internal/paste"
	"bingo/internal/server"
)

// stubRepo is a configurable paste.Repository for handler tests.
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

func newTestServer(t *testing.T, repo paste.Repository) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		BaseURL:           "https://example.com",
		MaxPasteSizeBytes: 5 * 1024 * 1024,
	}
	srv := server.New(cfg, nil, repo, nil, nil) // nil db: healthz ping skipped in unit tests
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

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

func TestHealthz_returns200(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	resp, err := http.Get(ts.URL + "/api/v1/healthz")
	if err != nil {
		t.Fatalf("GET /api/v1/healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`body["status"] = %q, want "ok"`, body["status"])
	}
}

func TestCreatePaste_success(t *testing.T) {
	repo := defaultRepo()
	repo.createFn = func(_ context.Context, p paste.CreateParams) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "abcd",
			Content:   p.Content,
			Language:  p.Language,
			SizeBytes: len(p.Content),
			ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
			CreatedAt: time.Now(),
		}, nil
	}
	ts := newTestServer(t, repo)

	body := `{"content":"hello world","language":"go","expires_in":"3mo"}`
	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/v1/pastes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["key"] != "abcd" {
		t.Errorf("key = %v, want abcd", got["key"])
	}
}

func TestCreatePaste_missingContent(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json",
		strings.NewReader(`{"language":"go","expires_in":"1d"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	var errResp struct {
		Error struct{ Code string } `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if errResp.Error.Code != "missing_content" {
		t.Errorf("error code = %q, want missing_content", errResp.Error.Code)
	}
}

func TestCreatePaste_invalidExpiresIn(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json",
		strings.NewReader(`{"content":"x","language":"go","expires_in":"forever"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreatePaste_unknownLanguage(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json",
		strings.NewReader(`{"content":"x","language":"notareal_language_xyz","expires_in":"1d"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	var errResp struct {
		Error struct{ Code string } `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&errResp) //nolint:errcheck
	if errResp.Error.Code != "unknown_language" {
		t.Errorf("error code = %q, want unknown_language", errResp.Error.Code)
	}
}

func TestCreatePaste_tooLarge(t *testing.T) {
	cfg := &config.Config{
		BaseURL:           "https://example.com",
		MaxPasteSizeBytes: 10, // tiny limit for test
	}
	srv := server.New(cfg, nil, defaultRepo(), nil, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	bigContent := bytes.Repeat([]byte("x"), 11)
	body, _ := json.Marshal(map[string]string{
		"content": string(bigContent), "language": "go", "expires_in": "1d",
	})
	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", resp.StatusCode)
	}
}

func TestCreatePaste_atSizeLimit(t *testing.T) {
	// Content at exactly MaxPasteSizeBytes must be accepted (returns 201, not 413).
	const limit = 10
	cfg := &config.Config{
		BaseURL:           "https://example.com",
		MaxPasteSizeBytes: limit,
	}
	repo := defaultRepo()
	repo.createFn = func(_ context.Context, p paste.CreateParams) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "limitkey",
			Content:   p.Content,
			Language:  p.Language,
			SizeBytes: len(p.Content),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
		}, nil
	}
	srv := server.New(cfg, nil, repo, nil, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	atLimitContent := bytes.Repeat([]byte("x"), limit)
	body, _ := json.Marshal(map[string]string{
		"content": string(atLimitContent), "language": "go", "expires_in": "1d",
	})
	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("content at limit: status = %d, want 201", resp.StatusCode)
	}
}

func TestGetPaste_success(t *testing.T) {
	expiry := time.Now().Add(24 * time.Hour).UTC()
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "abcd",
			Content:   "hello",
			Language:  "go",
			SizeBytes: 5,
			ExpiresAt: expiry,
			CreatedAt: time.Now().UTC(),
		}, nil
	}
	ts := newTestServer(t, repo)

	resp, err := http.Get(ts.URL + "/api/v1/pastes/abcd")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestGetPaste_notFound(t *testing.T) {
	ts := newTestServer(t, defaultRepo()) // defaultRepo returns ErrNotFound

	resp, err := http.Get(ts.URL + "/api/v1/pastes/missing")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
}

func TestGetPaste_expiredLazy(t *testing.T) {
	var deletedKey string
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "oldkey",
			Content:   "stale",
			Language:  "text",
			SizeBytes: 5,
			ExpiresAt: time.Now().Add(-1 * time.Hour), // already expired
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}, nil
	}
	repo.deleteFn = func(_ context.Context, key string) error {
		deletedKey = key
		return nil
	}
	ts := newTestServer(t, repo)

	resp, err := http.Get(ts.URL + "/api/v1/pastes/oldkey")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("lazy expiry: status = %d, want 204", resp.StatusCode)
	}
	if deletedKey != "oldkey" {
		t.Errorf("lazy expiry: Delete not called with correct key, got %q", deletedKey)
	}
}

func TestGetLanguages(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	resp, err := http.Get(ts.URL + "/api/v1/languages")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string][]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body["languages"]) == 0 {
		t.Error("languages list is empty")
	}
}

// Ensure sql import is used when db is non-nil in integration scenarios.
var _ *sql.DB

func TestGetPasteRaw_success(t *testing.T) {
	expiry := time.Now().Add(24 * time.Hour).UTC()
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "abcd",
			Content:   "package main\n",
			Language:  "go",
			SizeBytes: 14,
			ExpiresAt: expiry,
			CreatedAt: time.Now().UTC(),
		}, nil
	}
	ts := newTestServer(t, repo)

	resp, err := http.Get(ts.URL + "/api/v1/pastes/abcd/raw")
	if err != nil {
		t.Fatalf("GET raw: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", ct)
	}
	if xct := resp.Header.Get("X-Content-Type-Options"); xct != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", xct)
	}
}

func TestDeletePaste_success(t *testing.T) {
	var deletedKey string
	ownerID := int64(42)
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "mykey",
			OwnerID:   &ownerID,
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil
	}
	repo.deleteFn = func(_ context.Context, key string) error {
		deletedKey = key
		return nil
	}
	cfg := &config.Config{BaseURL: "https://example.com", MaxPasteSizeBytes: 5 * 1024 * 1024}
	srv := server.New(cfg, nil, repo, nil, nil)
	wrapped := injectSession(srv, &auth.Session{UserID: 42, Sub: "sub|42", Email: "a@b.com"})
	ts := httptest.NewServer(wrapped)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/pastes/mykey", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if deletedKey != "mykey" {
		t.Errorf("Delete called with key %q, want mykey", deletedKey)
	}
}

// injectSession wraps handler with middleware that forces a specific session into context.
func injectSession(next http.Handler, sess *auth.Session) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), auth.TestSessionKey{}, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestDeletePaste_anonymousPasteForbidden(t *testing.T) {
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "anon",
			OwnerID:   nil, // anonymous paste
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

func TestDeletePaste_wrongOwner(t *testing.T) {
	ownerID := int64(1)
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "owned",
			OwnerID:   &ownerID,
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil
	}
	cfg := &config.Config{BaseURL: "https://example.com", MaxPasteSizeBytes: 5 * 1024 * 1024}
	srv := server.New(cfg, nil, repo, nil, nil)
	wrapped := injectSession(srv, &auth.Session{UserID: 99, Sub: "sub|99", Email: "b@b.com"})
	ts := httptest.NewServer(wrapped)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/pastes/owned", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("wrong owner delete: status = %d, want 403", resp.StatusCode)
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

func TestListMyPastes_authenticated(t *testing.T) {
	repo := defaultRepo()
	repo.listByOwnerFn = func(_ context.Context, ownerID int64, _ int) ([]*paste.Paste, error) {
		return []*paste.Paste{
			{
				Key:       "abc",
				Content:   "hello",
				Language:  "go",
				SizeBytes: 5,
				ExpiresAt: time.Now().Add(time.Hour),
				CreatedAt: time.Now(),
				OwnerID:   &ownerID,
			},
		}, nil
	}
	cfg := &config.Config{BaseURL: "https://example.com", MaxPasteSizeBytes: 5 * 1024 * 1024}
	srv := server.New(cfg, nil, repo, nil, nil)
	wrapped := injectSession(srv, &auth.Session{UserID: 7, Sub: "sub|7", Email: "c@c.com"})
	ts := httptest.NewServer(wrapped)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/v1/pastes?mine=true")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pastes, ok := body["pastes"].([]any)
	if !ok || len(pastes) == 0 {
		t.Errorf("expected non-empty pastes list, got %v", body["pastes"])
	}
	if count, ok := body["count"].(float64); !ok || int(count) != 1 {
		t.Errorf("count = %v, want 1", body["count"])
	}
	// Verify the content field is not present in list items.
	if item, ok := pastes[0].(map[string]any); ok {
		if _, hasContent := item["content"]; hasContent {
			t.Error("list item must not include the content field")
		}
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

func TestSecurityHeaders(t *testing.T) {
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return nil, paste.ErrNotFound
	}
	ts := newTestServer(t, repo)

	resp, err := http.Get(ts.URL + "/api/v1/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	csp := resp.Header.Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'self'")
	assert.NotContains(t, csp, "unsafe-eval")
}

// TestRequireAuthMiddleware_authEnabled verifies that when auth is enabled,
// unauthenticated API requests return 401 and unauthenticated browser requests
// are redirected to /auth/login. Auth flow and healthz endpoints are exempt.
func TestRequireAuthMiddleware_authEnabled_apiReturns401(t *testing.T) {
	ts := newTestServerWithAuth(t, new(auth.Provider))

	resp, err := http.Get(ts.URL + "/api/v1/pastes/somekey")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRequireAuthMiddleware_authEnabled_browserRedirectsToLogin(t *testing.T) {
	ts := newTestServerWithAuth(t, new(auth.Provider))

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/auth/login", resp.Header.Get("Location"))
}

func TestRequireAuthMiddleware_authEnabled_healthzExempt(t *testing.T) {
	ts := newTestServerWithAuth(t, new(auth.Provider))

	resp, err := http.Get(ts.URL + "/api/v1/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRequireAuthMiddleware_authEnabled_loginExempt(t *testing.T) {
	ts := newTestServerWithAuth(t, new(auth.Provider))

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	// /auth/login with a real Provider will panic on AuthCodeURL (zero-value codec),
	// but the important thing is that it is NOT rejected by requireAuthMiddleware.
	// We just verify the response is not 401.
	resp, err := client.Get(ts.URL + "/auth/login")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRequireAuthMiddleware_authDisabled_allowsAll(t *testing.T) {
	ts := newTestServer(t, defaultRepo()) // nil auth

	resp, err := http.Get(ts.URL + "/api/v1/languages")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ─── handleMe ────────────────────────────────────────────────────────────────

func TestHandleMe_authDisabled(t *testing.T) {
	ts := newTestServer(t, defaultRepo()) // nil auth provider

	resp, err := http.Get(ts.URL + "/api/v1/me")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		AuthEnabled   bool   `json:"auth_enabled"`
		Authenticated bool   `json:"authenticated"`
		Email         string `json:"email"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.False(t, body.AuthEnabled)
	assert.False(t, body.Authenticated)
	assert.Empty(t, body.Email)
}

func TestHandleMe_authEnabled_unauthenticated(t *testing.T) {
	ts := newTestServerWithAuth(t, new(auth.Provider))

	resp, err := http.Get(ts.URL + "/api/v1/me")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		AuthEnabled   bool `json:"auth_enabled"`
		Authenticated bool `json:"authenticated"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.True(t, body.AuthEnabled)
	assert.False(t, body.Authenticated)
}

func TestHandleMe_authEnabled_authenticated(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://example.com", MaxPasteSizeBytes: 5 * 1024 * 1024}
	srv := server.New(cfg, nil, defaultRepo(), new(auth.Provider), new(auth.UserRepository))
	wrapped := injectSession(srv, &auth.Session{UserID: 1, Sub: "sub|1", Email: "user@example.com"})
	ts := httptest.NewServer(wrapped)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/v1/me")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		AuthEnabled   bool   `json:"auth_enabled"`
		Authenticated bool   `json:"authenticated"`
		Email         string `json:"email"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.True(t, body.AuthEnabled)
	assert.True(t, body.Authenticated)
	assert.Equal(t, "user@example.com", body.Email)
}

// ─── corsMiddleware ───────────────────────────────────────────────────────────

func TestCORSMiddleware_preflightReturns204(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/pastes", nil)
	req.Header.Set("Origin", "https://example.com")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestCORSMiddleware_matchingOriginSetsHeaders(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/healthz", nil)
	req.Header.Set("Origin", "https://example.com") // matches cfg.BaseURL
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_mismatchedOriginNoHeader(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/healthz", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}

// ─── handleHealthz ────────────────────────────────────────────────────────────

func TestHealthz_dbUnavailable_returns503(t *testing.T) {
	// Use a real but closed *sql.DB so PingContext returns an error.
	db, err := sql.Open("pgx", "postgres://invalid:invalid@localhost:1/nodb?sslmode=disable&connect_timeout=1")
	require.NoError(t, err)
	defer db.Close()

	cfg := &config.Config{BaseURL: "https://example.com", MaxPasteSizeBytes: 5 * 1024 * 1024}
	srv := server.New(cfg, db, defaultRepo(), nil, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/v1/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}
