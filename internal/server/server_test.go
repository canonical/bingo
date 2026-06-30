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

func newTestServer(t *testing.T, repo paste.Repository) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		BaseURL:           "https://example.com",
		MaxPasteSizeBytes: 5 * 1024 * 1024,
	}
	srv := server.New(cfg, nil, repo) // nil db: healthz ping skipped in unit tests
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
		strings.NewReader(`{"content":"x","language":"brainfuck","expires_in":"1d"}`))
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
	srv := server.New(cfg, nil, defaultRepo())
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

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGetPaste_expiredLazy(t *testing.T) {
	var deletedKey string
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "oldkey",
			Content:   "stale",
			Language:  "plaintext",
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

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("lazy expiry: status = %d, want 404", resp.StatusCode)
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
	repo := defaultRepo()
	repo.deleteFn = func(_ context.Context, key string) error {
		deletedKey = key
		return nil
	}
	ts := newTestServer(t, repo)

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
