package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bingo/internal/auth"
)

func TestGenerateToken_length(t *testing.T) {
	tok, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if len(tok) < 32 {
		t.Errorf("token length = %d, want >= 32", len(tok))
	}
}

func TestGenerateToken_unique(t *testing.T) {
	t1, _ := auth.GenerateToken()
	t2, _ := auth.GenerateToken()
	if t1 == t2 {
		t.Error("two tokens are equal — not random enough")
	}
}

func TestValidateCSRF_valid(t *testing.T) {
	tok := "abc123validtoken"
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", tok)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: tok})
	if !auth.ValidateCSRF(req) {
		t.Error("ValidateCSRF() = false for matching header and cookie, want true")
	}
}

func TestValidateCSRF_missingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "tok"})
	if auth.ValidateCSRF(req) {
		t.Error("ValidateCSRF() = true with no header, want false")
	}
}

func TestValidateCSRF_missingCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", "tok")
	if auth.ValidateCSRF(req) {
		t.Error("ValidateCSRF() = true with no cookie, want false")
	}
}

func TestValidateCSRF_mismatch(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", "header-value")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie-value"})
	if auth.ValidateCSRF(req) {
		t.Error("ValidateCSRF() = true with mismatched values, want false")
	}
}

func TestClearCSRFCookie(t *testing.T) {
	w := httptest.NewRecorder()
	auth.ClearCSRFCookie(w)

	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == "csrf_token" {
			found = true
			if c.MaxAge >= 0 {
				t.Errorf("csrf_token MaxAge = %d, want < 0 (clear)", c.MaxAge)
			}
			if c.Value != "" {
				t.Errorf("csrf_token value = %q, want empty", c.Value)
			}
		}
	}
	if !found {
		t.Error("ClearCSRFCookie() did not set a csrf_token cookie")
	}
}

func TestSetCSRFCookie_isNotHttpOnly(t *testing.T) {
	w := httptest.NewRecorder()
	auth.SetCSRFCookie(w, "mytoken")
	resp := w.Result()
	found := false
	for _, c := range resp.Cookies() {
		if c.Name == "csrf_token" {
			found = true
			if c.HttpOnly {
				t.Error("csrf_token cookie must NOT be HttpOnly (JS needs to read it)")
			}
			if c.Value != "mytoken" {
				t.Errorf("csrf_token value = %q, want mytoken", c.Value)
			}
		}
	}
	if !found {
		t.Error("csrf_token cookie not set")
	}
}
