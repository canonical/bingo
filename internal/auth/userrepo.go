package auth

import (
	"context"
	"database/sql"
	"fmt"
)

// UserRepository persists and looks up users by their OIDC sub claim.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a UserRepository backed by db.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// UpsertUser ensures a row exists in users for sub, updates email, and returns the id.
// This is idempotent: repeated calls with the same sub return the same id.
func (r *UserRepository) UpsertUser(ctx context.Context, sub, email string) (int64, error) {
	const q = `
		INSERT INTO users (sub, email)
		VALUES ($1, $2)
		ON CONFLICT (sub) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`
	var id int64
	if err := r.db.QueryRowContext(ctx, q, sub, email).Scan(&id); err != nil {
		return 0, fmt.Errorf("upsert user: %w", err)
	}
	return id, nil
}
