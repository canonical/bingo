// Package server implements the HTTP server, routing, and request handlers.
package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

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
	auth     *auth.Provider       // nil when auth is disabled
	userRepo *auth.UserRepository // nil when auth is disabled
	handler  http.Handler         // pre-built middleware chain
}

// New creates a Server with all API routes registered.
// authProvider and userRepo may be nil when authentication is disabled,
// but they must both be nil or both be non-nil.
// db may be nil in tests that do not exercise the healthz DB ping.
func New(cfg *config.Config, db *sql.DB, repo paste.Repository, authProvider *auth.Provider, userRepo *auth.UserRepository) *Server {
	if (authProvider == nil) != (userRepo == nil) {
		panic("server: authProvider and userRepo must both be nil or both be non-nil")
	}
	s := &Server{
		mux:      http.NewServeMux(),
		cfg:      cfg,
		db:       db,
		repo:     repo,
		auth:     authProvider,
		userRepo: userRepo,
	}
	s.registerRoutes()
	if cfg.WebDir != "" {
		s.serveStaticFiles(cfg.WebDir)
	}
	s.handler = s.corsMiddleware(s.auth.Middleware(s.mux))
	return s
}

// ServeHTTP implements http.Handler, delegating through the pre-built
// CORS + auth middleware chain to the internal ServeMux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
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

// serveStaticFiles registers a catch-all handler that serves web/dist as a SPA.
// Requests matching /api/* or /auth/* are not intercepted (already registered).
// Any path that doesn't resolve to an existing file falls back to index.html.
func (s *Server) serveStaticFiles(webDir string) {
	fs := http.FileServer(http.Dir(webDir))
	s.mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(webDir, filepath.Clean("/"+r.URL.Path))
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	}))
}
