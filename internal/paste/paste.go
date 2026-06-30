// Package paste defines paste domain types, the repository interface, and the language registry.
package paste

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrNotFound is returned when a paste key does not exist or has expired.
var ErrNotFound = errors.New("paste not found")

// Paste is the full paste entity as stored in the database.
type Paste struct {
	ID        int64
	Key       string
	Content   string
	Language  string
	Title     string
	SizeBytes int
	ExpiresAt time.Time
	CreatedAt time.Time
	OwnerID   *int64
}

// CreateParams holds the caller-supplied fields for creating a new paste.
type CreateParams struct {
	Content   string
	Language  string
	Title     string
	ExpiresIn ExpiresIn
	OwnerID   *int64 // nil for anonymous pastes
}

// Repository defines the storage operations for pastes.
// Implementations must be safe for concurrent use.
type Repository interface {
	// Create persists a new paste, generating a collision-resistant key internally.
	Create(ctx context.Context, params CreateParams) (*Paste, error)
	// GetByKey retrieves a paste by its short key. Returns ErrNotFound when absent.
	GetByKey(ctx context.Context, key string) (*Paste, error)
	// Delete removes a paste by key. A missing key is not an error.
	Delete(ctx context.Context, key string) error
	// DeleteExpired removes all pastes whose expires_at is in the past.
	// Returns the number of rows deleted.
	DeleteExpired(ctx context.Context) (int64, error)
	// ListByOwner returns up to limit active (non-expired) pastes for ownerID,
	// ordered by created_at descending.
	ListByOwner(ctx context.Context, ownerID int64, limit int) ([]*Paste, error)
}

// ExpiresIn is the set of allowed paste expiration durations.
type ExpiresIn string

const (
	ExpiresIn1d  ExpiresIn = "1d"
	ExpiresIn1w  ExpiresIn = "1w"
	ExpiresIn1mo ExpiresIn = "1mo"
	ExpiresIn3mo ExpiresIn = "3mo"
	ExpiresIn1y  ExpiresIn = "1y"
)

var expiryDurations = map[ExpiresIn]time.Duration{
	ExpiresIn1d:  24 * time.Hour,
	ExpiresIn1w:  7 * 24 * time.Hour,
	ExpiresIn1mo: 30 * 24 * time.Hour,
	ExpiresIn3mo: 90 * 24 * time.Hour,
	ExpiresIn1y:  365 * 24 * time.Hour,
}

// ParseExpiresIn validates and returns an ExpiresIn from a raw string.
// Valid values: "1d", "1w", "1mo", "3mo", "1y".
func ParseExpiresIn(s string) (ExpiresIn, error) {
	e := ExpiresIn(s)
	if _, ok := expiryDurations[e]; !ok {
		return "", fmt.Errorf("invalid expires_in %q: must be one of 1d, 1w, 1mo, 3mo, 1y", s)
	}
	return e, nil
}

// Duration returns the time.Duration for this ExpiresIn value.
func (e ExpiresIn) Duration() time.Duration {
	return expiryDurations[e]
}

// validLanguages is the set of accepted language identifiers.
// Keys match react-syntax-highlighter language names used by the frontend.
var validLanguages = map[string]struct{}{
	"plaintext":  {},
	"bash":       {},
	"c":          {},
	"cpp":        {},
	"css":        {},
	"diff":       {},
	"go":         {},
	"html":       {},
	"java":       {},
	"javascript": {},
	"json":       {},
	"markdown":   {},
	"python":     {},
	"ruby":       {},
	"rust":       {},
	"shell":      {},
	"sql":        {},
	"toml":       {},
	"typescript": {},
	"xml":        {},
	"yaml":       {},
}

// IsValidLanguage reports whether lang is in the supported language registry.
func IsValidLanguage(lang string) bool {
	_, ok := validLanguages[lang]
	return ok
}

// AllLanguages returns a sorted slice of all supported language identifiers.
func AllLanguages() []string {
	langs := make([]string, 0, len(validLanguages))
	for l := range validLanguages {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	return langs
}
