package server_test

import (
	"database/sql"
	"log"
	"os"
	"testing"

	"bingo/internal/database"
)

// testDB is a shared Postgres connection pool for integration tests in this
// package that need a real *auth.UserRepository (e.g. the OIDC callback
// flow). nil when DATABASE_URL is not set, in which case such tests are
// skipped via requireTestDB.
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

// requireTestDB skips the test if no DATABASE_URL was provided.
func requireTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if testDB == nil {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	return testDB
}
