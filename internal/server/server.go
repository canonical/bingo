// Package server implements the HTTP server, routing, and request handlers.
package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	s.handler = s.securityHeadersMiddleware(s.corsMiddleware(s.auth.Middleware(s.requireAuthMiddleware(s.mux))))
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

// securityHeadersMiddleware sets mandatory security headers on every response.
func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	const csp = "default-src 'self'; " +
		"style-src 'self' 'unsafe-inline' https://fonts.ubuntu.com; " +
		"font-src 'self' https://assets.ubuntu.com; " +
		"img-src 'self' data: https://assets.ubuntu.com; " +
		"object-src 'none'; " +
		"frame-ancestors 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}

// requireAuthMiddleware gates the entire application behind authentication when auth
// is enabled. Exempt paths (auth flow and healthz) are always allowed through.
// For API paths, unauthenticated requests receive a 401 JSON response.
// For browser paths, unauthenticated requests are redirected to the app's
// externally-visible /auth/login (prefixed with cfg.BasePath so the redirect
// resolves correctly behind path-prefix-stripping reverse proxies).
// When auth is disabled (s.auth == nil), all requests pass through unchanged.
func (s *Server) requireAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			next.ServeHTTP(w, r)
			return
		}
		// Auth flow endpoints, health check, and me endpoint are always accessible.
		p := r.URL.Path
		if p == "/auth/login" || p == "/auth/callback" || p == "/auth/logout" || p == "/api/v1/healthz" || p == "/api/v1/me" {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := auth.FromContext(r.Context()); !ok {
			if strings.HasPrefix(p, "/api/") {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "Login required.")
				return
			}
			http.Redirect(w, r, s.cfg.BasePath()+"/auth/login", http.StatusFound)
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
	s.mux.HandleFunc("GET /api/v1/me", s.handleMe)
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
// Any path that doesn't resolve to an existing file falls back to index.html,
// served with an injected <base> tag (see indexHTMLWithBase) so the SPA's
// relative asset/API references resolve against the app's externally
// visible base path rather than the proxy's domain root.
func (s *Server) serveStaticFiles(webDir string) {
	fs := http.FileServer(http.Dir(webDir))
	indexHTML, err := s.indexHTMLWithBase(webDir)
	if err != nil {
		slog.Error("read index.html", "err", err)
	}

	serveIndex := func(w http.ResponseWriter, r *http.Request) {
		if indexHTML == nil {
			// Fall back to the unmodified file if it couldn't be read/patched
			// at startup; this still works correctly when BasePath is "".
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	}

	s.mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean("/" + r.URL.Path)
		if clean == "/" || clean == "/index.html" {
			serveIndex(w, r)
			return
		}
		if _, err := os.Stat(filepath.Join(webDir, clean)); os.IsNotExist(err) {
			serveIndex(w, r)
			return
		}
		fs.ServeHTTP(w, r)
	}))
}

// indexHTMLWithBase reads webDir/index.html and injects a <base href> tag
// derived from cfg.BasePath (see config.Config.BasePath) right after <head>.
// Reverse proxies that route by path prefix (e.g. Traefik ingress-per-app in
// its default "path" mode) strip that prefix before forwarding to the app,
// so the built HTML's relative asset references (see vite.config.ts's
// `base: "./"`) and the frontend's relative API calls would otherwise
// resolve against the proxy's domain root instead of the app's externally
// visible base path. With the <base> tag in place, both resolve correctly
// under any prefix, including "" (domain root, e.g. local/non-charm runs).
func (s *Server) indexHTMLWithBase(webDir string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(webDir, "index.html"))
	if err != nil {
		return nil, err
	}
	baseTag := `<base href="` + s.cfg.BasePath() + `/">`
	return bytes.Replace(data, []byte("<head>"), []byte("<head>"+baseTag), 1), nil
}
