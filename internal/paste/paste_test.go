package paste_test

import (
	"testing"
	"time"

	"bingo/internal/paste"
)

func TestParseExpiresIn_valid(t *testing.T) {
	tests := []struct {
		input    string
		wantDur  time.Duration
	}{
		{"1d", 24 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"1mo", 30 * 24 * time.Hour},
		{"3mo", 90 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			e, err := paste.ParseExpiresIn(tt.input)
			if err != nil {
				t.Fatalf("ParseExpiresIn(%q) error = %v", tt.input, err)
			}
			if got := e.Duration(); got != tt.wantDur {
				t.Errorf("Duration() = %v, want %v", got, tt.wantDur)
			}
		})
	}
}

func TestParseExpiresIn_invalid(t *testing.T) {
	for _, s := range []string{"", "2d", "forever", "1month", "0"} {
		t.Run(s, func(t *testing.T) {
			_, err := paste.ParseExpiresIn(s)
			if err == nil {
				t.Errorf("ParseExpiresIn(%q) expected error, got nil", s)
			}
		})
	}
}

func TestIsValidLanguage(t *testing.T) {
	if !paste.IsValidLanguage("python") {
		t.Error("IsValidLanguage(\"python\") = false, want true")
	}
	if !paste.IsValidLanguage("cobol") {
		t.Error("IsValidLanguage(\"cobol\") = false, want true")
	}
	if !paste.IsValidLanguage("typescript") {
		t.Error("IsValidLanguage(\"typescript\") = false, want true")
	}
	if paste.IsValidLanguage("plaintext") {
		t.Error("IsValidLanguage(\"plaintext\") = true, want false")
	}
	if paste.IsValidLanguage("") {
		t.Error("IsValidLanguage(\"\") = true, want false")
	}
}

func TestAllLanguages_notEmpty(t *testing.T) {
	langs := paste.AllLanguages()
	if len(langs) == 0 {
		t.Error("AllLanguages() returned empty slice")
	}
	found := false
	for _, l := range langs {
		if l == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AllLanguages() does not include \"go\"")
	}
}
