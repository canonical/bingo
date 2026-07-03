package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bingo/internal/auth"
	"bingo/internal/config"
	"bingo/internal/paste"
)

// newCSRFTestServer builds a Server with a non-nil auth.Provider (zero-value is
// enough — requireCSRF only checks s.auth == nil and then calls auth.ValidateCSRF).
// We bypass New() to avoid calling Provider.Middleware, which would panic on a
// zero-value codec.
func newCSRFTestServer(t *testing.T, repo paste.Repository) *Server {
	t.Helper()
	if repo == nil {
		repo = &stubCSRFRepo{}
	}
	cfg := &config.Config{
		BaseURL:           "https://example.com",
		MaxPasteSizeBytes: 5 * 1024 * 1024,
	}
	s := &Server{
		mux:  http.NewServeMux(),
		cfg:  cfg,
		repo: repo,
		auth: new(auth.Provider), // non-nil: triggers CSRF enforcement
	}
	s.registerRoutes()
	s.handler = s.mux
	return s
}

// stubCSRFRepo is a minimal paste.Repository whose methods all panic when called,
// used to confirm that CSRF-rejected requests never reach the repository.
type stubCSRFRepo struct{}

func (r *stubCSRFRepo) Create(_ context.Context, _ paste.CreateParams) (*paste.Paste, error) {
	panic("unexpected Create call")
}
func (r *stubCSRFRepo) GetByKey(_ context.Context, _ string) (*paste.Paste, error) {
	panic("unexpected GetByKey call")
}
func (r *stubCSRFRepo) Delete(_ context.Context, _ string) error {
	panic("unexpected Delete call")
}
func (r *stubCSRFRepo) DeleteExpired(_ context.Context) (int64, error) {
	panic("unexpected DeleteExpired call")
}
func (r *stubCSRFRepo) ListByOwner(_ context.Context, _ int64, _ int) ([]*paste.Paste, error) {
	panic("unexpected ListByOwner call")
}

// stubRepoWithCreate is a stub that allows Create to succeed, used for testing
// unauthenticated requests that should reach the handler.
type stubRepoWithCreate struct{}

func (r *stubRepoWithCreate) Create(_ context.Context, p paste.CreateParams) (*paste.Paste, error) {
	return &paste.Paste{
		Key:      "test-key",
		OwnerID:  p.OwnerID,
		Content:  p.Content,
		Language: p.Language,
		Title:    p.Title,
	}, nil
}
func (r *stubRepoWithCreate) GetByKey(_ context.Context, _ string) (*paste.Paste, error) {
	panic("unexpected GetByKey call")
}
func (r *stubRepoWithCreate) Delete(_ context.Context, _ string) error {
	panic("unexpected Delete call")
}
func (r *stubRepoWithCreate) DeleteExpired(_ context.Context) (int64, error) {
	panic("unexpected DeleteExpired call")
}
func (r *stubRepoWithCreate) ListByOwner(_ context.Context, _ int64, _ int) ([]*paste.Paste, error) {
	panic("unexpected ListByOwner call")
}

func TestCreatePaste_authEnabled_missingCSRF_403(t *testing.T) {
	s := newCSRFTestServer(t, nil)

	body := `{"content":"hello","language":"go","expires_in":"1d"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/pastes", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	
	// Inject a session so the request is authenticated
	ctx := context.WithValue(r.Context(), auth.TestSessionKey{}, &auth.Session{UserID: 1, Sub: "s", Email: "e@e.com"})
	r = r.WithContext(ctx)
	
	// No X-CSRF-Token header and no csrf_token cookie → should be rejected.
	w := httptest.NewRecorder()

	s.handleCreatePaste(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("authenticated request missing CSRF on createPaste: status = %d, want 403", w.Code)
	}
}

func TestDeletePaste_authEnabled_missingCSRF_403(t *testing.T) {
	s := newCSRFTestServer(t, nil)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/pastes/somekey", nil)
	
	// Inject a session so the request is authenticated
	ctx := context.WithValue(r.Context(), auth.TestSessionKey{}, &auth.Session{UserID: 1, Sub: "s", Email: "e@e.com"})
	r = r.WithContext(ctx)
	
	// No X-CSRF-Token header and no csrf_token cookie → should be rejected.
	w := httptest.NewRecorder()

	s.handleDeletePaste(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("authenticated request missing CSRF on deletePaste: status = %d, want 403", w.Code)
	}
}

func TestCreatePaste_authEnabled_unauthenticated_401(t *testing.T) {
	s := newCSRFTestServer(t, &stubRepoWithCreate{})

	body := `{"content":"hello","language":"go","expires_in":"1d"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/pastes", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	// Don't inject a session — request is unauthenticated.
	// When auth is enabled, unauthenticated requests must be rejected with 401.
	w := httptest.NewRecorder()

	s.handleCreatePaste(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated request to createPaste (auth enabled): status = %d, want 401", w.Code)
	}
}
