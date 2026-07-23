package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateCallbackState_noCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/auth/callback?state=abc123", nil)
	if validateCallbackState(r) {
		t.Error("expected false: no oidc_state cookie present")
	}
}

func TestValidateCallbackState_mismatch(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/auth/callback?state=abc123", nil)
	r.AddCookie(&http.Cookie{Name: "oidc_state", Value: "different"})
	if validateCallbackState(r) {
		t.Error("expected false: cookie value does not match state query param")
	}
}

func TestValidateCallbackState_emptyCookieValue(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/auth/callback?state=abc123", nil)
	r.AddCookie(&http.Cookie{Name: "oidc_state", Value: ""})
	if validateCallbackState(r) {
		t.Error("expected false: cookie value is empty")
	}
}

func TestValidateCallbackState_match(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/auth/callback?state=abc123", nil)
	r.AddCookie(&http.Cookie{Name: "oidc_state", Value: "abc123"})
	if !validateCallbackState(r) {
		t.Error("expected true: cookie matches state query param")
	}
}
