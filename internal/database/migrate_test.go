package database_test

import (
	"os"
	"testing"

	"bingo/internal/database"
)

func TestMigrate(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	db, err := database.Open(dbURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	// First run applies all pending migrations.
	if err := database.Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Second run against an up-to-date schema must be a no-op (ErrNoChange
	// swallowed internally), not an error.
	if err := database.Migrate(db); err != nil {
		t.Fatalf("Migrate() (idempotent re-run) error = %v", err)
	}

	// Confirm the expected tables exist.
	for _, table := range []string{"pastes", "users", "schema_migrations"} {
		var exists bool
		err := db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`,
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("query table %q existence: %v", table, err)
		}
		if !exists {
			t.Errorf("table %q does not exist after Migrate()", table)
		}
	}
}
