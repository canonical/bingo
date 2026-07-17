package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bingo/internal/auth"
)

func TestCodec_roundtrip(t *testing.T) {
	codec := auth.NewCodec("test-secret-key-that-is-long-enough!")

	sess := &auth.Session{UserID: 42, Sub: "sub|abc123", Email: "user@example.com"}

	w := httptest.NewRecorder()
	if err := codec.Set(w, sess); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Build a request with the cookie from the response.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w.Result().Cookies() {
		req.AddCookie(c)
	}

	got, ok := codec.Read(req)
	if !ok {
		t.Fatal("Read() returned false, want true")
	}
	if got.UserID != sess.UserID {
		t.Errorf("UserID = %d, want %d", got.UserID, sess.UserID)
	}
	if got.Sub != sess.Sub {
		t.Errorf("Sub = %q, want %q", got.Sub, sess.Sub)
	}
	if got.Email != sess.Email {
		t.Errorf("Email = %q, want %q", got.Email, sess.Email)
	}
}

func TestCodec_Read_noCookie(t *testing.T) {
	codec := auth.NewCodec("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, ok := codec.Read(req)
	if ok {
		t.Error("Read() with no cookie = true, want false")
	}
}

func TestCodec_Read_tamperedCookie(t *testing.T) {
	codec := auth.NewCodec("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "bingo_session", Value: "tampered!!"})
	_, ok := codec.Read(req)
	if ok {
		t.Error("Read() with tampered cookie = true, want false")
	}
}

func TestCodec_Clear(t *testing.T) {
	codec := auth.NewCodec("test-secret")
	sess := &auth.Session{UserID: 1, Sub: "sub|x", Email: "x@example.com"}

	w := httptest.NewRecorder()
	if err := codec.Set(w, sess); err != nil {
		t.Fatalf("Set(): %v", err)
	}

	// Set then Clear.
	w2 := httptest.NewRecorder()
	codec.Clear(w2)

	// The clear response sets the cookie with MaxAge=-1.
	found := false
	for _, c := range w2.Result().Cookies() {
		if c.Name == "bingo_session" && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("Clear() did not set bingo_session cookie with MaxAge < 0")
	}
}
