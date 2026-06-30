package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

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

func (s *Server) handleCreatePaste(w http.ResponseWriter, r *http.Request) {
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

// getPasteOrError fetches a paste, applies lazy expiry, and writes an error
// response when the paste is missing or expired. Returns non-nil err when
// the caller should stop processing.
func (s *Server) getPasteOrError(w http.ResponseWriter, ctx context.Context, k string) (*paste.Paste, error) {
	p, err := s.repo.GetByKey(ctx, k)
	if err != nil {
		if errors.Is(err, paste.ErrNotFound) {
			writeError(w, http.StatusNotFound, "paste_not_found", "Paste not found.")
			return nil, err
		}
		slog.Error("get paste", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return nil, err
	}
	// Lazy expiry: delete and return 404 if the paste has expired.
	if time.Now().After(p.ExpiresAt) {
		_ = s.repo.Delete(ctx, k)
		writeError(w, http.StatusNotFound, "paste_not_found", "Paste not found.")
		return nil, errors.New("expired")
	}
	return p, nil
}

func (s *Server) handleDeletePaste(w http.ResponseWriter, r *http.Request) {
	k := r.PathValue("key")
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
