package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bingo/internal/config"
	"bingo/internal/server"
)

// newTestServer creates a test HTTP server backed by a Server with a minimal config.
// It registers ts.Close via t.Cleanup so callers don't need to defer.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := &config.Config{BaseURL: "https://example.com"}
	srv := server.New(cfg)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

func TestHealthz_returns200WithJSONBody(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/healthz")
	if err != nil {
		t.Fatalf("GET /api/v1/healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Error("Content-Type header missing")
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`body["status"] = %q, want "ok"`, body["status"])
	}
}

func TestStubRoutes_return501(t *testing.T) {
	ts := newTestServer(t)

	tests := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/pastes"},
		{"GET", "/api/v1/pastes/abc123"},
		{"GET", "/api/v1/pastes/abc123/raw"},
		{"DELETE", "/api/v1/pastes/abc123"},
		{"GET", "/api/v1/languages"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, ts.URL+tt.path, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNotImplemented {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotImplemented)
			}
			if ct := resp.Header.Get("Content-Type"); ct == "" {
				t.Errorf("Content-Type missing for %s %s", tt.method, tt.path)
			}
		})
	}
}
