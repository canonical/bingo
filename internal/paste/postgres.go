package paste

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"bingo/internal/key"
)

// PostgresRepository implements Repository against a PostgreSQL database.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository creates a PostgresRepository backed by the given connection pool.
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Create generates a unique key and inserts a new paste row.
// On a UNIQUE key collision (pgcode 23505), it retries with a longer key.
func (r *PostgresRepository) Create(ctx context.Context, params CreateParams) (*Paste, error) {
	expiresAt := time.Now().UTC().Add(params.ExpiresIn.Duration())
	keyLen := 4

	for range 10 {
		k := key.GenerateKey(keyLen)
		p, err := r.insert(ctx, k, expiresAt, params)
		if err == nil {
			return p, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Unique key collision: try a longer key.
			keyLen++
			continue
		}
		return nil, fmt.Errorf("insert paste: %w", err)
	}
	return nil, fmt.Errorf("failed to generate unique key after 10 attempts")
}

func (r *PostgresRepository) insert(ctx context.Context, k string, expiresAt time.Time, params CreateParams) (*Paste, error) {
	const q = `
		INSERT INTO pastes (key, content, language, title, size_bytes, expires_at, owner_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, key, content, language, title, size_bytes, expires_at, created_at, owner_id`

	var title sql.NullString
	if params.Title != "" {
		title = sql.NullString{String: params.Title, Valid: true}
	}

	row := r.db.QueryRowContext(ctx, q,
		k, params.Content, params.Language, title,
		len(params.Content), expiresAt, params.OwnerID,
	)
	return scanPaste(row)
}

// GetByKey retrieves a paste by key. Returns ErrNotFound when no row exists or the paste is expired.
// Expired pastes are lazily deleted before returning ErrNotFound.
func (r *PostgresRepository) GetByKey(ctx context.Context, k string) (*Paste, error) {
	const q = `
		SELECT id, key, content, language, title, size_bytes, expires_at, created_at, owner_id
		FROM pastes WHERE key = $1`
	row := r.db.QueryRowContext(ctx, q, k)
	p, err := scanPaste(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if time.Now().After(p.ExpiresAt) {
		_ = r.Delete(ctx, k)
		return nil, ErrNotFound
	}
	return p, nil
}

// Delete removes a paste by key. A missing key is silently ignored.
func (r *PostgresRepository) Delete(ctx context.Context, k string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM pastes WHERE key = $1`, k)
	return err
}

// DeleteExpired removes all pastes whose expires_at is before now.
// Returns the number of rows deleted.
func (r *PostgresRepository) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM pastes WHERE expires_at < now()`)
	if err != nil {
		return 0, fmt.Errorf("delete expired: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}

// ListByOwner returns up to limit active pastes owned by ownerID, newest first.
func (r *PostgresRepository) ListByOwner(ctx context.Context, ownerID int64, limit int) ([]*Paste, error) {
	const q = `
		SELECT id, key, content, language, title, size_bytes, expires_at, created_at, owner_id
		FROM pastes
		WHERE owner_id = $1 AND expires_at > now()
		ORDER BY created_at DESC
		LIMIT $2`
	rows, err := r.db.QueryContext(ctx, q, ownerID, limit)
	if err != nil {
		return nil, fmt.Errorf("list by owner: %w", err)
	}
	defer rows.Close()

	var pastes []*Paste
	for rows.Next() {
		var p Paste
		var title sql.NullString
		err := rows.Scan(
			&p.ID, &p.Key, &p.Content, &p.Language, &title,
			&p.SizeBytes, &p.ExpiresAt, &p.CreatedAt, &p.OwnerID,
		)
		if err != nil {
			return nil, fmt.Errorf("scan paste: %w", err)
		}
		if title.Valid {
			p.Title = title.String
		}
		pastes = append(pastes, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return pastes, nil
}

// scanPaste reads a single paste row from a *sql.Row.
func scanPaste(row *sql.Row) (*Paste, error) {
	var p Paste
	var title sql.NullString
	err := row.Scan(
		&p.ID, &p.Key, &p.Content, &p.Language, &title,
		&p.SizeBytes, &p.ExpiresAt, &p.CreatedAt, &p.OwnerID,
	)
	if err != nil {
		return nil, err
	}
	if title.Valid {
		p.Title = title.String
	}
	return &p, nil
}
