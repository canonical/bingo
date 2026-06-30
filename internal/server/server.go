// Package server implements the HTTP server, routing, and request handlers.
package server

import (
	"encoding/json"
	"net/http"

	"bingo/internal/config"
)

// Server holds the HTTP router and application configuration.
type Server struct {
	mux *http.ServeMux
	cfg *config.Config
}

// New creates a Server with all API routes registered.
func New(cfg *config.Config) *Server {
	s := &Server{
		mux: http.NewServeMux(),
		cfg: cfg,
	}
	s.registerRoutes()
	return s
}

// ServeHTTP implements http.Handler, delegating to the internal ServeMux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// registerRoutes wires all API endpoints to their handler methods.
// Go 1.22 enhanced ServeMux: method prefix and {key} path parameters are supported.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/v1/healthz", s.handleHealthz)
	s.mux.HandleFunc("POST /api/v1/pastes", s.handleCreatePaste)
	s.mux.HandleFunc("GET /api/v1/pastes/{key}", s.handleGetPaste)
	s.mux.HandleFunc("GET /api/v1/pastes/{key}/raw", s.handleGetPasteRaw)
	s.mux.HandleFunc("DELETE /api/v1/pastes/{key}", s.handleDeletePaste)
	s.mux.HandleFunc("GET /api/v1/languages", s.handleListLanguages)
}

// writeJSON sets Content-Type, writes the status code, and JSON-encodes v.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errResponse is the standard error envelope for all 4xx/5xx responses.
type errResponse struct {
	Error errDetail `json:"error"`
}

type errDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeNotImplemented(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotImplemented, errResponse{
		Error: errDetail{
			Code:    "not_implemented",
			Message: "Not implemented",
		},
	})
}

// handleHealthz returns 200 OK with {"status":"ok"}.
// Phase 2 will extend this to also ping the database.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleCreatePaste handles POST /api/v1/pastes.
// Implemented in Phase 2.
func (s *Server) handleCreatePaste(w http.ResponseWriter, _ *http.Request) {
	writeNotImplemented(w)
}

// handleGetPaste handles GET /api/v1/pastes/{key}.
// Implemented in Phase 2.
func (s *Server) handleGetPaste(w http.ResponseWriter, _ *http.Request) {
	writeNotImplemented(w)
}

// handleGetPasteRaw handles GET /api/v1/pastes/{key}/raw.
// Implemented in Phase 2.
func (s *Server) handleGetPasteRaw(w http.ResponseWriter, _ *http.Request) {
	writeNotImplemented(w)
}

// handleDeletePaste handles DELETE /api/v1/pastes/{key}.
// Implemented in Phase 2.
func (s *Server) handleDeletePaste(w http.ResponseWriter, _ *http.Request) {
	writeNotImplemented(w)
}

// handleListLanguages handles GET /api/v1/languages.
// Implemented in Phase 2.
func (s *Server) handleListLanguages(w http.ResponseWriter, _ *http.Request) {
	writeNotImplemented(w)
}
