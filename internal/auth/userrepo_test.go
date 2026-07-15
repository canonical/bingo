package auth_test

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"

	"bingo/internal/auth"
	"bingo/internal/database"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		os.Exit(m.Run())
	}
	var err error
	testDB, err = database.Open(dbURL)
	if err != nil {
		log.Fatalf("open test db: %v", err)
	}
	if err := database.Migrate(testDB); err != nil {
		log.Fatalf("migrate test db: %v", err)
	}
	os.Exit(m.Run())
}

func requireDB(t *testing.T) *auth.UserRepository {
	t.Helper()
	if testDB == nil {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	return auth.NewUserRepository(testDB)
}

func TestUserRepository_UpsertUser(t *testing.T) {
	repo := requireDB(t)
	ctx := context.Background()

	sub := "sub|upsert-test-unique"
	email := "upsert@example.com"

	// First upsert — creates.
	id1, err := repo.UpsertUser(ctx, sub, email)
	if err != nil {
		t.Fatalf("UpsertUser() first call error = %v", err)
	}
	if id1 <= 0 {
		t.Errorf("UpsertUser() first call id = %d, want > 0", id1)
	}

	// Second upsert — idempotent, returns same id.
	id2, err := repo.UpsertUser(ctx, sub, email)
	if err != nil {
		t.Fatalf("UpsertUser() second call error = %v", err)
	}
	if id2 != id1 {
		t.Errorf("UpsertUser() second call id = %d, want %d (same)", id2, id1)
	}

	// Cleanup.
	testDB.ExecContext(ctx, "DELETE FROM users WHERE sub = $1", sub) //nolint:errcheck
}

func TestUserRepository_UpsertUser_queryError(t *testing.T) {
	repo := requireDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already-cancelled context: the query must fail

	_, err := repo.UpsertUser(ctx, "sub|cancelled", "cancelled@example.com")
	if err == nil {
		t.Fatal("UpsertUser() with a cancelled context: want error, got nil")
	}
}
