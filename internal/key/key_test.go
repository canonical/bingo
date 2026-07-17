package key_test

import (
	"strings"
	"testing"

	"bingo/internal/key"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func TestGenerateKey_length(t *testing.T) {
	for _, n := range []int{4, 5, 8, 32} {
		t.Run("len"+string(rune('0'+n)), func(t *testing.T) {
			got := key.GenerateKey(n)
			if len(got) != n {
				t.Errorf("GenerateKey(%d) length = %d, want %d", n, len(got), n)
			}
		})
	}
}

func TestGenerateKey_charset(t *testing.T) {
	for range 100 {
		k := key.GenerateKey(16)
		for i, c := range k {
			if !strings.ContainsRune(alphabet, c) {
				t.Errorf("GenerateKey(16)[%d] = %q, not in base62 alphabet", i, c)
			}
		}
	}
}

func TestGenerateKey_uniqueness(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for range 1000 {
		k := key.GenerateKey(8)
		if seen[k] {
			t.Errorf("GenerateKey(8) produced duplicate: %q", k)
		}
		seen[k] = true
	}
}
