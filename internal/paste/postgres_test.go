package paste_test

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"bingo/internal/database"
	"bingo/internal/paste"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// No DB configured: run unit-only tests (paste_test.go) and exit.
		os.Exit(m.Run())
	}

	var err error
	testDB, err = database.Open(dbURL)
	if err != nil {
		log.Fatalf("open test db: %v", err)
	}
	defer testDB.Close()

	if err := database.Migrate(testDB); err != nil {
		log.Fatalf("migrate test db: %v", err)
	}

	os.Exit(m.Run())
}

// requireDB skips the test if no DATABASE_URL was provided.
func requireDB(t *testing.T) *paste.PostgresRepository {
	t.Helper()
	if testDB == nil {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	return paste.NewPostgresRepository(testDB)
}

// cleanPastes removes all rows from pastes between tests to ensure isolation.
func cleanPastes(t *testing.T) {
	t.Helper()
	if _, err := testDB.ExecContext(context.Background(), "DELETE FROM pastes"); err != nil {
		t.Fatalf("clean pastes: %v", err)
	}
}

func TestPostgresRepository_Create(t *testing.T) {
	repo := requireDB(t)
	t.Cleanup(func() { cleanPastes(t) })

	params := paste.CreateParams{
		Content:   "hello world",
		Language:  "plaintext",
		Title:     "test paste",
		ExpiresIn: paste.ExpiresIn3mo,
	}

	p, err := repo.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if len(p.Key) < 4 {
		t.Errorf("Key length = %d, want >= 4", len(p.Key))
	}
	if p.Content != params.Content {
		t.Errorf("Content = %q, want %q", p.Content, params.Content)
	}
	if p.Language != params.Language {
		t.Errorf("Language = %q, want %q", p.Language, params.Language)
	}
	if p.Title != params.Title {
		t.Errorf("Title = %q, want %q", p.Title, params.Title)
	}
	if p.SizeBytes != len(params.Content) {
		t.Errorf("SizeBytes = %d, want %d", p.SizeBytes, len(params.Content))
	}
	if p.OwnerID != nil {
		t.Errorf("OwnerID = %v, want nil", p.OwnerID)
	}
	if p.ExpiresAt.Before(time.Now()) {
		t.Errorf("ExpiresAt %v is in the past", p.ExpiresAt)
	}
}

func TestPostgresRepository_GetByKey(t *testing.T) {
	repo := requireDB(t)
	t.Cleanup(func() { cleanPastes(t) })

	created, err := repo.Create(context.Background(), paste.CreateParams{
		Content:   "get test",
		Language:  "go",
		ExpiresIn: paste.ExpiresIn1d,
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	got, err := repo.GetByKey(context.Background(), created.Key)
	if err != nil {
		t.Fatalf("GetByKey(): %v", err)
	}

	if diff := cmp.Diff(created, got, cmpopts.IgnoreFields(paste.Paste{}, "CreatedAt", "ExpiresAt")); diff != "" {
		t.Errorf("GetByKey() mismatch (-want +got):\n%s", diff)
	}
}

func TestPostgresRepository_GetByKey_notFound(t *testing.T) {
	repo := requireDB(t)

	_, err := repo.GetByKey(context.Background(), "nosuchkey")
	if err == nil {
		t.Fatal("GetByKey() expected error, got nil")
	}
	if err != paste.ErrNotFound {
		t.Errorf("GetByKey() error = %v, want ErrNotFound", err)
	}
}

func TestPostgresRepository_Delete(t *testing.T) {
	repo := requireDB(t)
	t.Cleanup(func() { cleanPastes(t) })

	p, err := repo.Create(context.Background(), paste.CreateParams{
		Content:   "delete me",
		Language:  "plaintext",
		ExpiresIn: paste.ExpiresIn1d,
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}

	if err := repo.Delete(context.Background(), p.Key); err != nil {
		t.Fatalf("Delete(): %v", err)
	}

	_, err = repo.GetByKey(context.Background(), p.Key)
	if err != paste.ErrNotFound {
		t.Errorf("after Delete, GetByKey error = %v, want ErrNotFound", err)
	}

	// Test Delete on non-existent key (must not error)
	if err := repo.Delete(context.Background(), "does-not-exist-key"); err != nil {
		t.Errorf("Delete non-existent key error = %v, want nil", err)
	}
}

func TestPostgresRepository_DeleteExpired(t *testing.T) {
	repo := requireDB(t)
	t.Cleanup(func() { cleanPastes(t) })

	// Insert a row with expires_at already in the past via raw SQL
	// (bypasses the expiry_after_creation constraint by using a past timestamp at insert time)
	// We use a small offset to satisfy the DB constraint (expires_at > created_at)
	// by inserting created_at even further in the past.
	_, err := testDB.ExecContext(context.Background(),
		`INSERT INTO pastes (key, content, language, size_bytes, expires_at, created_at)
         VALUES ($1, $2, $3, $4, now() - interval '1 second', now() - interval '2 seconds')`,
		"expiredkey", "old content", "plaintext", 11,
	)
	if err != nil {
		t.Fatalf("insert expired paste: %v", err)
	}

	// Insert a live paste to confirm it is not deleted
	live, err := repo.Create(context.Background(), paste.CreateParams{
		Content:   "live paste",
		Language:  "plaintext",
		ExpiresIn: paste.ExpiresIn1y,
	})
	if err != nil {
		t.Fatalf("Create live paste: %v", err)
	}

	n, err := repo.DeleteExpired(context.Background())
	if err != nil {
		t.Fatalf("DeleteExpired(): %v", err)
	}
	if n < 1 {
		t.Errorf("DeleteExpired() = %d, want >= 1", n)
	}

	// Expired paste should be gone
	if _, err := repo.GetByKey(context.Background(), "expiredkey"); err != paste.ErrNotFound {
		t.Errorf("expired paste still present after DeleteExpired")
	}

	// Live paste should survive
	if _, err := repo.GetByKey(context.Background(), live.Key); err != nil {
		t.Errorf("live paste deleted unexpectedly: %v", err)
	}
}
