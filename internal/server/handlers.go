package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"bingo/internal/auth"
	"bingo/internal/paste"
)

// handleHealthz returns {"status":"ok"} and, when a DB is wired, pings it.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if s.db != nil {
		if err := s.db.PingContext(r.Context()); err != nil {
			slog.Error("healthz db ping failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "db_unavailable", "Database is unavailable.")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// createRequest is the JSON body for POST /api/v1/pastes.
type createRequest struct {
	Content   string `json:"content"`
	Language  string `json:"language"`
	Title     string `json:"title"`
	ExpiresIn string `json:"expires_in"`
}

// createResponse is the JSON body for a 201 Created response.
type createResponse struct {
	Key       string    `json:"key"`
	URL       string    `json:"url"`
	RawURL    string    `json:"raw_url"`
	Language  string    `json:"language"`
	Title     string    `json:"title,omitempty"`
	SizeBytes int       `json:"size_bytes"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// requireCSRF validates the CSRF token when auth is enabled.
// Returns true if the request may proceed; writes a 403 and returns false otherwise.
func (s *Server) requireCSRF(w http.ResponseWriter, r *http.Request) bool {
	if s.auth == nil {
		return true // CSRF only matters when auth is enabled
	}
	if !auth.ValidateCSRF(r) {
		writeError(w, http.StatusForbidden, "csrf_invalid", "CSRF token missing or invalid")
		return false
	}
	return true
}

// requireAuth enforces mandatory authentication when auth is enabled.
// Returns true if the request may proceed; writes a 401 and returns false otherwise.
// When auth is disabled (s.auth == nil), all requests are allowed through as anonymous.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.auth == nil {
		return true // auth disabled: all pastes are anonymous
	}
	if _, ok := auth.FromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Login required.")
		return false
	}
	return true
}

func (s *Server) handleCreatePaste(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if !s.requireCSRF(w, r) {
		return
	}
	// Limit body size before any read to prevent memory exhaustion.
	// Use MaxPasteSizeBytes + overhead for JSON framing; the precise content
	// size check below enforces the exact limit on the content field itself.
	const jsonOverhead = 4096
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxPasteSizeBytes+jsonOverhead)

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "content_too_large",
				"Paste exceeds the configured size limit.")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body is not valid JSON.")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "missing_content", "Content must not be empty.")
		return
	}
	if int64(len(req.Content)) > s.cfg.MaxPasteSizeBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "content_too_large",
			"Paste exceeds the configured size limit.")
		return
	}
	if req.Language == "" {
		req.Language = "plaintext"
	}
	if !paste.IsValidLanguage(req.Language) {
		writeError(w, http.StatusBadRequest, "unknown_language",
			"Language is not in the supported registry.")
		return
	}

	expiresIn := req.ExpiresIn
	if expiresIn == "" {
		expiresIn = string(paste.ExpiresIn3mo)
	}
	ei, err := paste.ParseExpiresIn(expiresIn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_expires_in",
			"expires_in must be one of: 1d, 1w, 1mo, 3mo, 1y.")
		return
	}

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
	if err != nil {
		slog.Error("create paste", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return
	}

	writeJSON(w, http.StatusCreated, createResponse{
		Key:       p.Key,
		URL:       s.cfg.BaseURL + "/" + p.Key,
		RawURL:    s.cfg.BaseURL + "/api/v1/pastes/" + p.Key + "/raw",
		Language:  p.Language,
		Title:     p.Title,
		SizeBytes: p.SizeBytes,
		ExpiresAt: p.ExpiresAt.UTC(),
		CreatedAt: p.CreatedAt.UTC(),
	})
}

// pasteResponse is returned by GET /api/v1/pastes/{key}.
type pasteResponse struct {
	Key       string    `json:"key"`
	URL       string    `json:"url"`
	RawURL    string    `json:"raw_url"`
	Content   string    `json:"content"`
	Language  string    `json:"language"`
	Title     string    `json:"title,omitempty"`
	SizeBytes int       `json:"size_bytes"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) handleGetPaste(w http.ResponseWriter, r *http.Request) {
	k := r.PathValue("key")
	p, err := s.getPasteOrError(w, r.Context(), k)
	if err != nil {
		return // response already written
	}
	writeJSON(w, http.StatusOK, s.toPasteResponse(p))
}

func (s *Server) handleGetPasteRaw(w http.ResponseWriter, r *http.Request) {
	k := r.PathValue("key")
	p, err := s.getPasteOrError(w, r.Context(), k)
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(p.Content))
}

// getPasteOrError fetches a paste, applies lazy expiry, and writes a 204 No
// Content response when the paste is missing or expired. Returns non-nil err
// when the caller should stop processing.
func (s *Server) getPasteOrError(w http.ResponseWriter, ctx context.Context, k string) (*paste.Paste, error) {
	p, err := s.repo.GetByKey(ctx, k)
	if err != nil {
		if errors.Is(err, paste.ErrNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return nil, err
		}
		slog.Error("get paste", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return nil, err
	}
	// Lazy expiry: delete and return 204 if the paste has expired.
	if time.Now().After(p.ExpiresAt) {
		_ = s.repo.Delete(ctx, k)
		w.WriteHeader(http.StatusNoContent)
		return nil, errors.New("expired")
	}
	return p, nil
}

func (s *Server) handleDeletePaste(w http.ResponseWriter, r *http.Request) {
	if !s.requireCSRF(w, r) {
		return
	}
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

func (s *Server) handleListLanguages(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]string{"languages": paste.AllLanguages()})
}

// meResponse is returned by GET /api/v1/me.
type meResponse struct {
	AuthEnabled   bool   `json:"auth_enabled"`
	Authenticated bool   `json:"authenticated"`
	Email         string `json:"email,omitempty"`
}

// handleMe reports the current session state.
// Always accessible (exempt from requireAuthMiddleware) so the frontend can
// determine whether authentication is required without being redirected.
// - Auth disabled: {"auth_enabled":false,"authenticated":false}
// - Auth enabled, unauthenticated: {"auth_enabled":true,"authenticated":false}
// - Auth enabled, authenticated: {"auth_enabled":true,"authenticated":true,"email":"..."}
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	authEnabled := s.auth != nil
	sess, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusOK, meResponse{AuthEnabled: authEnabled, Authenticated: false})
		return
	}
	writeJSON(w, http.StatusOK, meResponse{AuthEnabled: authEnabled, Authenticated: true, Email: sess.Email})
}

// pasteListItem is a summary view of a paste used in list responses (no content field).
type pasteListItem struct {
	Key       string    `json:"key"`
	URL       string    `json:"url"`
	Language  string    `json:"language"`
	Title     string    `json:"title,omitempty"`
	SizeBytes int       `json:"size_bytes"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// listMyPastesResponse is the envelope returned by GET /api/v1/pastes?mine=true.
type listMyPastesResponse struct {
	Pastes []pasteListItem `json:"pastes"`
	Count  int             `json:"count"`
}

// handleListMyPastes returns the authenticated user's pastes.
// Requires an active session; returns 401 when the user is not authenticated.
func (s *Server) handleListMyPastes(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("mine") != "true" {
		writeError(w, http.StatusBadRequest, "invalid_request", "This endpoint requires ?mine=true.")
		return
	}

	sess, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Login required.")
		return
	}
	pastes, err := s.repo.ListByOwner(r.Context(), sess.UserID, 50)
	if err != nil {
		slog.Error("list pastes by owner", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return
	}
	items := make([]pasteListItem, 0, len(pastes))
	for _, p := range pastes {
		items = append(items, pasteListItem{
			Key:       p.Key,
			URL:       s.cfg.BaseURL + "/" + p.Key,
			Language:  p.Language,
			Title:     p.Title,
			SizeBytes: p.SizeBytes,
			ExpiresAt: p.ExpiresAt.UTC(),
			CreatedAt: p.CreatedAt.UTC(),
		})
	}
	writeJSON(w, http.StatusOK, listMyPastesResponse{Pastes: items, Count: len(items)})
}

func (s *Server) toPasteResponse(p *paste.Paste) pasteResponse {
	return pasteResponse{
		Key:       p.Key,
		URL:       s.cfg.BaseURL + "/" + p.Key,
		RawURL:    s.cfg.BaseURL + "/api/v1/pastes/" + p.Key + "/raw",
		Content:   p.Content,
		Language:  p.Language,
		Title:     p.Title,
		SizeBytes: p.SizeBytes,
		ExpiresAt: p.ExpiresAt.UTC(),
		CreatedAt: p.CreatedAt.UTC(),
	}
}
