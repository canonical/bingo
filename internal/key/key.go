// Package key provides base62 key generation for paste identifiers.
package key

import (
	"crypto/rand"
	"math/big"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateKey returns a cryptographically random n-character base62 string.
// The alphabet is 0-9, A-Z, a-z (62 characters).
func GenerateKey(n int) string {
	b := make([]byte, n)
	alphabetLen := big.NewInt(int64(len(alphabet)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			panic("key: crypto/rand failed: " + err.Error())
		}
		b[i] = alphabet[idx.Int64()]
	}
	return string(b)
}
