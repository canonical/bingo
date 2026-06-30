// Package server implements the HTTP server, routing, and request handlers.
package server

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"bingo/internal/config"
	"bingo/internal/paste"
)

// Server holds the HTTP router, application configuration, and storage.
type Server struct {
	mux  *http.ServeMux
	cfg  *config.Config
	db   *sql.DB
	repo paste.Repository
}

// New creates a Server with all API routes registered.
// db may be nil in tests that do not exercise the healthz DB ping.
func New(cfg *config.Config, db *sql.DB, repo paste.Repository) *Server {
	s := &Server{
		mux:  http.NewServeMux(),
		cfg:  cfg,
		db:   db,
		repo: repo,
	}
	s.registerRoutes()
	return s
}

// ServeHTTP implements http.Handler, delegating to the internal ServeMux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// registerRoutes wires all API endpoints to their handler methods.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/v1/healthz", s.handleHealthz)
	s.mux.HandleFunc("POST /api/v1/pastes", s.handleCreatePaste)
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
