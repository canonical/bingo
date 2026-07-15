package database_test

import (
	"os"
	"testing"

	"bingo/internal/database"
)

func TestOpen_success(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	db, err := database.Open(dbURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("Ping() after Open() error = %v", err)
	}
}
