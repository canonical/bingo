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
