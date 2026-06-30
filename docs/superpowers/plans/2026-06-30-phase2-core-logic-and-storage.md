# Phase 2: Core Logic & Storage — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement paste creation, retrieval, deletion, and expiry using real PostgreSQL storage behind a repository interface, with base62 key generation, database migrations, a background expiry sweep, full input validation, and wired HTTP handlers.

**Architecture:** A pure key-generation package (`internal/key`) feeds into a PostgreSQL repository (`internal/paste/postgres.go`) that implements the `paste.Repository` interface. The `internal/database` package owns connection-pool setup and schema migration via `golang-migrate` with embedded SQL files. The HTTP server's `New()` constructor gains a `paste.Repository` argument so all handlers are wired to real storage. A background goroutine in `internal/paste/sweep.go` periodically purges expired rows; handlers also do lazy expiry on `GET`.

**Tech Stack:** `pgx/v5` via `database/sql` stdlib adapter, `golang-migrate/v4` (iofs source + postgres driver), `github.com/google/go-cmp` for struct comparison in tests, standard `testing` package, `net/http/httptest` for handler tests.

## Global Constraints

- Module path: `bingo`; Go 1.22+
- DB driver: `github.com/jackc/pgx/v5/stdlib` registered as `"pgx"` — **not** `lib/pq`
- Migrations: `golang-migrate/migrate/v4` with embedded SQL files via `//go:embed` + `iofs`
- Repository pattern: `paste.Repository` interface; implementation in `postgres.go`; no ORM
- All DB calls use Context variants (`QueryContext`, `ExecContext`, `QueryRowContext`)
- Parameterized queries only — `$1`, `$2` placeholders; **never** string interpolation
- Connection pool: `SetMaxOpenConns(25)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(5 * time.Minute)`
- `defer rows.Close()` on every rows result
- Key generation: base62 alphabet (`0-9A-Za-z`), start at 4 chars, retry with length+1 on `UNIQUE` violation (pgconn error code `"23505"`), max 10 attempts
- Expiry durations: `1d` (24h), `1w` (7d), `1mo` (30d), `3mo` (90d, default), `1y` (365d) — no other values valid
- Content size limit: `MAX_PASTE_SIZE_BYTES` from config (default 5 MiB = 5242880 bytes); enforce at handler boundary → 413
- Language validation: reject unknown language keys → 400 `unknown_language`
- Error codes (JSON envelope `{"error":{"code":"...","message":"..."}}`): `missing_content`, `invalid_expires_in`, `unknown_language`, `content_too_large`, `paste_not_found`, `forbidden`, `internal_error`
- Lazy expiry on GET: if `expires_at` is in the past, delete and return 404
- Background sweep interval: configurable; use `1 * time.Hour` as default in main
- Integration tests: require `DATABASE_URL` env var; skip with `t.Skip()` when absent
- `gofmt`-clean, `golangci-lint run ./...` must pass, conventional commits, Co-authored-by Copilot trailer
- `github.com/google/go-cmp` for struct comparison in tests

## Global Constraints — DB Schema

```sql
CREATE TABLE pastes (
    id         BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key        TEXT        NOT NULL UNIQUE,
    content    TEXT        NOT NULL,
    language   VARCHAR(64) NOT NULL DEFAULT 'plaintext',
    title      VARCHAR(255),
    size_bytes INTEGER     NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    owner_id   BIGINT,
    CONSTRAINT size_positive         CHECK (size_bytes >= 1),
    CONSTRAINT key_length            CHECK (char_length(key) BETWEEN 4 AND 32),
    CONSTRAINT expiry_after_creation CHECK (expires_at > created_at)
);
CREATE INDEX pastes_expires_at_idx ON pastes (expires_at);
CREATE INDEX pastes_owner_id_idx   ON pastes (owner_id);
```

---

## File Map

| File | Responsibility |
|------|---------------|
| `internal/key/key.go` | `GenerateKey(n int) string` — base62, crypto-random |
| `internal/key/key_test.go` | Length, charset, and statistical uniqueness tests |
| `internal/paste/paste.go` | `Paste` struct, `CreateParams`, `Repository` interface, `ExpiresIn` type, language registry, `ErrNotFound` |
| `internal/paste/paste_test.go` | Unit tests for `ParseExpiresIn`, `IsValidLanguage` |
| `internal/paste/postgres.go` | `PostgresRepository`: Create (collision retry), GetByKey, Delete, DeleteExpired |
| `internal/paste/postgres_test.go` | `TestMain` + integration tests (skipped if no `DATABASE_URL`) |
| `internal/paste/sweep.go` | `StartSweep(ctx, repo, interval) func()` |
| `internal/paste/sweep_test.go` | Mock-repo sweep tests |
| `internal/database/db.go` | `Open(databaseURL string) (*sql.DB, error)` + pool config |
| `internal/database/migrate.go` | `Migrate(db *sql.DB) error` — golang-migrate + iofs |
| `internal/database/migrations/001_create_pastes.up.sql` | Create pastes table + indexes |
| `internal/database/migrations/001_create_pastes.down.sql` | Drop pastes table |
| `internal/server/server.go` | Update `Server` to hold `repo paste.Repository` + `db *sql.DB`; update `New()` signature |
| `internal/server/handlers.go` | All 6 handler methods extracted from server.go with full implementations |
| `internal/server/server_test.go` | Update: add mock repo, handler behaviour tests |
| `cmd/bingo/main.go` | Wire DB open → migrate → repo → server(repo) → sweep |

---

## Task 1: Base62 Key Generation

**Files:**
- Modify: `internal/key/key.go` (replace stub)
- Create: `internal/key/key_test.go`

**Interfaces:**
- Produces: `func GenerateKey(n int) string` — returns a random `n`-character string over `0-9A-Za-z`

- [ ] **Step 1: Write the failing test**

Create `internal/key/key_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./internal/key/... -v
```

Expected: `FAIL` — `key.GenerateKey` undefined (package is a stub).

- [ ] **Step 3: Implement `internal/key/key.go`**

Replace the stub entirely:
```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./internal/key/... -v
```

Expected:
```
--- PASS: TestGenerateKey_length (0.00s)
--- PASS: TestGenerateKey_charset (0.00s)
--- PASS: TestGenerateKey_uniqueness (0.00s)
PASS
ok      bingo/internal/key
```

- [ ] **Step 5: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
git add internal/key/
git commit -m "feat(key): implement base62 key generation

- crypto/rand-backed selection from 62-char alphabet (0-9A-Za-z)
- GenerateKey(n int) string for arbitrary key length
- Tests: length, charset, 1000-sample uniqueness check

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 2: Paste Domain Types, ExpiresIn & Language Registry

**Files:**
- Modify: `internal/paste/paste.go` (replace stub with full domain types)
- Create: `internal/paste/paste_test.go`

**Interfaces:**
- Produces:
  - `type Paste struct` with fields: `ID int64`, `Key string`, `Content string`, `Language string`, `Title string`, `SizeBytes int`, `ExpiresAt time.Time`, `CreatedAt time.Time`, `OwnerID *int64`
  - `type CreateParams struct` with fields: `Content string`, `Language string`, `Title string`, `ExpiresIn ExpiresIn`, `OwnerID *int64`
  - `type Repository interface` with methods: `Create(ctx, CreateParams) (*Paste, error)`, `GetByKey(ctx, key string) (*Paste, error)`, `Delete(ctx, key string) error`, `DeleteExpired(ctx) (int64, error)`
  - `type ExpiresIn string`; constants: `ExpiresIn1d`, `ExpiresIn1w`, `ExpiresIn1mo`, `ExpiresIn3mo`, `ExpiresIn1y`
  - `func ParseExpiresIn(s string) (ExpiresIn, error)`
  - `func (e ExpiresIn) Duration() time.Duration`
  - `func AllLanguages() []string`
  - `func IsValidLanguage(lang string) bool`
  - `var ErrNotFound = errors.New("paste not found")`

- [ ] **Step 1: Write the failing tests**

Create `internal/paste/paste_test.go`:
```go
package paste_test

import (
	"testing"
	"time"

	"bingo/internal/paste"
)

func TestParseExpiresIn_valid(t *testing.T) {
	tests := []struct {
		input    string
		wantDur  time.Duration
	}{
		{"1d", 24 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"1mo", 30 * 24 * time.Hour},
		{"3mo", 90 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			e, err := paste.ParseExpiresIn(tt.input)
			if err != nil {
				t.Fatalf("ParseExpiresIn(%q) error = %v", tt.input, err)
			}
			if got := e.Duration(); got != tt.wantDur {
				t.Errorf("Duration() = %v, want %v", got, tt.wantDur)
			}
		})
	}
}

func TestParseExpiresIn_invalid(t *testing.T) {
	for _, s := range []string{"", "2d", "forever", "1month", "0"} {
		t.Run(s, func(t *testing.T) {
			_, err := paste.ParseExpiresIn(s)
			if err == nil {
				t.Errorf("ParseExpiresIn(%q) expected error, got nil", s)
			}
		})
	}
}

func TestIsValidLanguage(t *testing.T) {
	if !paste.IsValidLanguage("python") {
		t.Error("IsValidLanguage(\"python\") = false, want true")
	}
	if !paste.IsValidLanguage("plaintext") {
		t.Error("IsValidLanguage(\"plaintext\") = false, want true")
	}
	if paste.IsValidLanguage("cobol") {
		t.Error("IsValidLanguage(\"cobol\") = true, want false")
	}
	if paste.IsValidLanguage("") {
		t.Error("IsValidLanguage(\"\") = true, want false")
	}
}

func TestAllLanguages_notEmpty(t *testing.T) {
	langs := paste.AllLanguages()
	if len(langs) == 0 {
		t.Error("AllLanguages() returned empty slice")
	}
	found := false
	for _, l := range langs {
		if l == "plaintext" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AllLanguages() does not include \"plaintext\"")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./internal/paste/... -v -run "TestParseExpiresIn|TestIsValidLanguage|TestAllLanguages"
```

Expected: `FAIL` — `paste.ParseExpiresIn` undefined.

- [ ] **Step 3: Implement `internal/paste/paste.go`**

Replace the stub entirely:
```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./internal/paste/... -v -run "TestParseExpiresIn|TestIsValidLanguage|TestAllLanguages"
```

Expected: all named tests PASS.

- [ ] **Step 5: Run full suite for regressions**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./...
```

Expected: all existing tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
git add internal/paste/paste.go internal/paste/paste_test.go
git commit -m "feat(paste): add domain types, ExpiresIn, language registry, Repository interface

- Paste struct and CreateParams with all schema fields
- Repository interface: Create, GetByKey, Delete, DeleteExpired
- ExpiresIn type with ParseExpiresIn + Duration() (1d/1w/1mo/3mo/1y)
- Language registry: 21 languages (plaintext default)
- ErrNotFound sentinel for missing/expired pastes
- Table-driven unit tests for ExpiresIn and language validation

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 3: External Dependencies & Database Package

**Files:**
- Modify: `go.mod`, `go.sum` (add pgx/v5, golang-migrate, go-cmp)
- Modify: `internal/database/db.go` (replace stub with real implementation)
- Create: `internal/database/migrate.go`
- Create: `internal/database/migrations/001_create_pastes.up.sql`
- Create: `internal/database/migrations/001_create_pastes.down.sql`

**Interfaces:**
- Produces:
  - `func Open(databaseURL string) (*sql.DB, error)` — opens pgx connection with pool config
  - `func Migrate(db *sql.DB) error` — runs embedded migrations; no-op if already current

- [ ] **Step 1: Add external dependencies**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go get github.com/jackc/pgx/v5
go get github.com/golang-migrate/migrate/v4
go get github.com/google/go-cmp/cmp
go mod tidy
```

Expected: `go.mod` gains new `require` entries; `go.sum` is generated.

- [ ] **Step 2: Create migration SQL files**

Create `internal/database/migrations/001_create_pastes.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS pastes (
    id         BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key        TEXT        NOT NULL UNIQUE,
    content    TEXT        NOT NULL,
    language   VARCHAR(64) NOT NULL DEFAULT 'plaintext',
    title      VARCHAR(255),
    size_bytes INTEGER     NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    owner_id   BIGINT,

    CONSTRAINT size_positive         CHECK (size_bytes >= 1),
    CONSTRAINT key_length            CHECK (char_length(key) BETWEEN 4 AND 32),
    CONSTRAINT expiry_after_creation CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS pastes_expires_at_idx ON pastes (expires_at);
CREATE INDEX IF NOT EXISTS pastes_owner_id_idx   ON pastes (owner_id);
```

Create `internal/database/migrations/001_create_pastes.down.sql`:
```sql
DROP TABLE IF EXISTS pastes;
```

- [ ] **Step 3: Implement `internal/database/db.go`**

Replace the stub entirely:
```go
// Package database provides PostgreSQL connection pool helpers and migration support.
package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
)

// Open opens a PostgreSQL connection pool using the pgx driver.
// The returned *sql.DB is configured for production-grade connection pooling.
func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}
```

- [ ] **Step 4: Implement `internal/database/migrate.go`**

Create `internal/database/migrate.go`:
```go
package database

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"database/sql"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate runs all pending up-migrations against db.
// It is idempotent: re-running against an up-to-date schema is a no-op.
func Migrate(db *sql.DB) error {
	srcDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}
	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", srcDriver, "postgres", dbDriver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Verify compilation**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go build ./...
```

Expected: no output, exit code 0.

- [ ] **Step 6: Run all tests**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./...
```

Expected: `internal/config` and `internal/server` PASS; `internal/database` reports no test files (integration test added in Task 4).

- [ ] **Step 7: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
git add go.mod go.sum internal/database/
git commit -m "feat(database): add pgx/v5 connection pool, golang-migrate, and schema migrations

- database.Open(): pgx driver, pool (25 open / 5 idle / 5min lifetime)
- database.Migrate(): golang-migrate with embedded SQL via iofs
- 001_create_pastes.up.sql: pastes table with all constraints and indexes
- 001_create_pastes.down.sql: drop table
- Dependencies: pgx/v5, golang-migrate/v4, google/go-cmp

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 4: PostgreSQL Repository

**Files:**
- Create: `internal/paste/postgres.go`
- Create: `internal/paste/postgres_test.go`

**Interfaces:**
- Consumes:
  - `bingo/internal/key.GenerateKey(n int) string`
  - `bingo/internal/paste.Paste`, `CreateParams`, `ErrNotFound`, `Repository`
  - `database/sql.DB`
  - `github.com/jackc/pgx/v5/pgconn.PgError` (unique violation code `"23505"`)
- Produces:
  - `type PostgresRepository struct` — implements `paste.Repository`
  - `func NewPostgresRepository(db *sql.DB) *PostgresRepository`
  - Methods: `Create`, `GetByKey`, `Delete`, `DeleteExpired`

- [ ] **Step 1: Write the failing integration tests**

Create `internal/paste/postgres_test.go`:
```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./internal/paste/... -v -run "TestPostgresRepository"
```

Expected: `FAIL` — `paste.PostgresRepository` and `paste.NewPostgresRepository` undefined.

- [ ] **Step 3: Implement `internal/paste/postgres.go`**

Create `internal/paste/postgres.go`:
```go
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

// GetByKey retrieves a paste by key. Returns ErrNotFound when no row exists.
func (r *PostgresRepository) GetByKey(ctx context.Context, k string) (*Paste, error) {
	const q = `
		SELECT id, key, content, language, title, size_bytes, expires_at, created_at, owner_id
		FROM pastes WHERE key = $1`
	row := r.db.QueryRowContext(ctx, q, k)
	p, err := scanPaste(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
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
```

- [ ] **Step 4: Run integration tests** (requires DATABASE_URL)

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
DATABASE_URL="postgres://user:pass@localhost/bingo_test?sslmode=disable" \
  go test ./internal/paste/... -v -run "TestPostgresRepository"
```

If `DATABASE_URL` is not available in this environment, run without it — unit tests from Task 2 still pass, integration tests are skipped:

```bash
go test ./internal/paste/... -v
```

Expected without DATABASE_URL:
```
--- PASS: TestParseExpiresIn_valid (0.00s)
--- PASS: TestParseExpiresIn_invalid (0.00s)
--- PASS: TestIsValidLanguage (0.00s)
--- PASS: TestAllLanguages_notEmpty (0.00s)
PASS
ok      bingo/internal/paste
```

- [ ] **Step 5: Run full suite**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./...
```

Expected: all previously passing tests still PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
git add internal/paste/postgres.go internal/paste/postgres_test.go
git commit -m "feat(paste): add PostgresRepository with collision-retry key generation

- NewPostgresRepository(*sql.DB) implements paste.Repository
- Create: generates base62 key (start len 4, +1 on pgcode 23505), RETURNING all fields
- GetByKey: returns ErrNotFound for missing rows
- Delete: silently ignores missing keys
- DeleteExpired: bulk delete by expires_at < now(), returns count
- Integration tests: TestMain checks DATABASE_URL, skips if absent
- go-cmp for struct comparison in GetByKey round-trip test

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 5: Background Expiry Sweep

**Files:**
- Create: `internal/paste/sweep.go`
- Create: `internal/paste/sweep_test.go`

**Interfaces:**
- Consumes: `paste.Repository` interface (`DeleteExpired` method)
- Produces: `func StartSweep(ctx context.Context, repo Repository, interval time.Duration) func()` — returns a cancel function that stops the goroutine

- [ ] **Step 1: Write the failing tests**

Create `internal/paste/sweep_test.go`:
```go
package paste_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"bingo/internal/paste"
)

// mockRepo is a minimal Repository implementation for sweep tests.
type mockRepo struct {
	deleteExpiredCalls atomic.Int64
}

func (m *mockRepo) Create(_ context.Context, _ paste.CreateParams) (*paste.Paste, error) {
	panic("not used in sweep tests")
}
func (m *mockRepo) GetByKey(_ context.Context, _ string) (*paste.Paste, error) {
	panic("not used in sweep tests")
}
func (m *mockRepo) Delete(_ context.Context, _ string) error {
	panic("not used in sweep tests")
}
func (m *mockRepo) DeleteExpired(_ context.Context) (int64, error) {
	m.deleteExpiredCalls.Add(1)
	return 0, nil
}

func TestStartSweep_callsDeleteExpired(t *testing.T) {
	repo := &mockRepo{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := paste.StartSweep(ctx, repo, 10*time.Millisecond)
	defer stop()

	// Wait long enough for at least 2 ticks.
	time.Sleep(50 * time.Millisecond)
	stop()

	if got := repo.deleteExpiredCalls.Load(); got < 2 {
		t.Errorf("DeleteExpired called %d times, want >= 2", got)
	}
}

func TestStartSweep_stopsOnContextCancel(t *testing.T) {
	repo := &mockRepo{}
	ctx, cancel := context.WithCancel(context.Background())

	stop := paste.StartSweep(ctx, repo, 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)

	cancel() // cancel parent context
	callsAtCancel := repo.deleteExpiredCalls.Load()
	time.Sleep(30 * time.Millisecond)
	callsAfterCancel := repo.deleteExpiredCalls.Load()
	stop()

	if callsAfterCancel > callsAtCancel+1 {
		t.Errorf("sweep continued after context cancel: %d calls before, %d after",
			callsAtCancel, callsAfterCancel)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./internal/paste/... -v -run "TestStartSweep"
```

Expected: `FAIL` — `paste.StartSweep` undefined.

- [ ] **Step 3: Implement `internal/paste/sweep.go`**

Create `internal/paste/sweep.go`:
```go
package paste

import (
	"context"
	"log/slog"
	"time"
)

// StartSweep starts a background goroutine that calls repo.DeleteExpired
// every interval. It returns a cancel function; calling it stops the goroutine.
// The goroutine also exits when ctx is cancelled.
func StartSweep(ctx context.Context, repo Repository, interval time.Duration) func() {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := repo.DeleteExpired(ctx)
				if err != nil {
					slog.Error("expiry sweep failed", "err", err)
					continue
				}
				if n > 0 {
					slog.Info("expiry sweep deleted expired pastes", "count", n)
				}
			}
		}
	}()
	return cancel
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./internal/paste/... -v -run "TestStartSweep"
```

Expected:
```
--- PASS: TestStartSweep_callsDeleteExpired (0.00s)
--- PASS: TestStartSweep_stopsOnContextCancel (0.00s)
PASS
```

- [ ] **Step 5: Run full suite**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
git add internal/paste/sweep.go internal/paste/sweep_test.go
git commit -m "feat(paste): add background expiry sweep goroutine

- StartSweep(ctx, repo, interval): ticker-based goroutine, cancellable
- Logs deleted count when > 0; logs error and continues on failure
- Tests: verifies >= 2 DeleteExpired calls in 50ms; verifies stop on cancel

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 6: HTTP Handlers — Full Implementation & Input Validation

**Files:**
- Modify: `internal/server/server.go` (add `repo`, `db` fields; update `New()`; remove stub handlers)
- Create: `internal/server/handlers.go` (all 6 handler methods with real logic)
- Modify: `internal/server/server_test.go` (add mock repo, update existing tests, add handler behaviour tests)

**Interfaces:**
- Consumes:
  - `paste.Repository` interface (Create, GetByKey, Delete)
  - `paste.ParseExpiresIn`, `paste.IsValidLanguage`, `paste.AllLanguages`, `paste.ErrNotFound`
  - `paste.CreateParams`, `paste.Paste`
  - `config.Config.MaxPasteSizeBytes`, `config.Config.BaseURL`
  - `*sql.DB` for healthz ping
- Produces: updated `func New(cfg *config.Config, db *sql.DB, repo paste.Repository) *Server`

- [ ] **Step 1: Write the failing tests**

Update `internal/server/server_test.go` — replace its entire content:
```go
package server_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bingo/internal/config"
	"bingo/internal/paste"
	"bingo/internal/server"
)

// stubRepo is a configurable paste.Repository for handler tests.
type stubRepo struct {
	createFn       func(ctx context.Context, p paste.CreateParams) (*paste.Paste, error)
	getByKeyFn     func(ctx context.Context, key string) (*paste.Paste, error)
	deleteFn       func(ctx context.Context, key string) error
	deleteExpiredFn func(ctx context.Context) (int64, error)
}

func (s *stubRepo) Create(ctx context.Context, p paste.CreateParams) (*paste.Paste, error) {
	return s.createFn(ctx, p)
}
func (s *stubRepo) GetByKey(ctx context.Context, key string) (*paste.Paste, error) {
	return s.getByKeyFn(ctx, key)
}
func (s *stubRepo) Delete(ctx context.Context, key string) error {
	return s.deleteFn(ctx, key)
}
func (s *stubRepo) DeleteExpired(ctx context.Context) (int64, error) {
	return s.deleteExpiredFn(ctx)
}

func newTestServer(t *testing.T, repo paste.Repository) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		BaseURL:           "https://example.com",
		MaxPasteSizeBytes: 5 * 1024 * 1024,
	}
	srv := server.New(cfg, nil, repo) // nil db: healthz ping skipped in unit tests
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

func defaultRepo() *stubRepo {
	return &stubRepo{
		createFn: func(_ context.Context, _ paste.CreateParams) (*paste.Paste, error) {
			return nil, nil
		},
		getByKeyFn: func(_ context.Context, _ string) (*paste.Paste, error) {
			return nil, paste.ErrNotFound
		},
		deleteFn: func(_ context.Context, _ string) error { return nil },
		deleteExpiredFn: func(_ context.Context) (int64, error) { return 0, nil },
	}
}

func TestHealthz_returns200(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	resp, err := http.Get(ts.URL + "/api/v1/healthz")
	if err != nil {
		t.Fatalf("GET /api/v1/healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`body["status"] = %q, want "ok"`, body["status"])
	}
}

func TestCreatePaste_success(t *testing.T) {
	repo := defaultRepo()
	repo.createFn = func(_ context.Context, p paste.CreateParams) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "abcd",
			Content:   p.Content,
			Language:  p.Language,
			SizeBytes: len(p.Content),
			ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
			CreatedAt: time.Now(),
		}, nil
	}
	ts := newTestServer(t, repo)

	body := `{"content":"hello world","language":"go","expires_in":"3mo"}`
	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/v1/pastes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["key"] != "abcd" {
		t.Errorf("key = %v, want abcd", got["key"])
	}
}

func TestCreatePaste_missingContent(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json",
		strings.NewReader(`{"language":"go","expires_in":"1d"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	var errResp struct {
		Error struct{ Code string } `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if errResp.Error.Code != "missing_content" {
		t.Errorf("error code = %q, want missing_content", errResp.Error.Code)
	}
}

func TestCreatePaste_invalidExpiresIn(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json",
		strings.NewReader(`{"content":"x","language":"go","expires_in":"forever"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreatePaste_unknownLanguage(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json",
		strings.NewReader(`{"content":"x","language":"brainfuck","expires_in":"1d"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	var errResp struct {
		Error struct{ Code string } `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&errResp) //nolint:errcheck
	if errResp.Error.Code != "unknown_language" {
		t.Errorf("error code = %q, want unknown_language", errResp.Error.Code)
	}
}

func TestCreatePaste_tooLarge(t *testing.T) {
	cfg := &config.Config{
		BaseURL:           "https://example.com",
		MaxPasteSizeBytes: 10, // tiny limit for test
	}
	srv := server.New(cfg, nil, defaultRepo())
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	bigContent := bytes.Repeat([]byte("x"), 11)
	body, _ := json.Marshal(map[string]string{
		"content": string(bigContent), "language": "go", "expires_in": "1d",
	})
	resp, err := http.Post(ts.URL+"/api/v1/pastes", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", resp.StatusCode)
	}
}

func TestGetPaste_success(t *testing.T) {
	expiry := time.Now().Add(24 * time.Hour).UTC()
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "abcd",
			Content:   "hello",
			Language:  "go",
			SizeBytes: 5,
			ExpiresAt: expiry,
			CreatedAt: time.Now().UTC(),
		}, nil
	}
	ts := newTestServer(t, repo)

	resp, err := http.Get(ts.URL + "/api/v1/pastes/abcd")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestGetPaste_notFound(t *testing.T) {
	ts := newTestServer(t, defaultRepo()) // defaultRepo returns ErrNotFound

	resp, err := http.Get(ts.URL + "/api/v1/pastes/missing")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGetPaste_expiredLazy(t *testing.T) {
	var deletedKey string
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, _ string) (*paste.Paste, error) {
		return &paste.Paste{
			Key:       "oldkey",
			Content:   "stale",
			Language:  "plaintext",
			SizeBytes: 5,
			ExpiresAt: time.Now().Add(-1 * time.Hour), // already expired
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}, nil
	}
	repo.deleteFn = func(_ context.Context, key string) error {
		deletedKey = key
		return nil
	}
	ts := newTestServer(t, repo)

	resp, err := http.Get(ts.URL + "/api/v1/pastes/oldkey")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("lazy expiry: status = %d, want 404", resp.StatusCode)
	}
	if deletedKey != "oldkey" {
		t.Errorf("lazy expiry: Delete not called with correct key, got %q", deletedKey)
	}
}

func TestGetLanguages(t *testing.T) {
	ts := newTestServer(t, defaultRepo())

	resp, err := http.Get(ts.URL + "/api/v1/languages")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string][]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body["languages"]) == 0 {
		t.Error("languages list is empty")
	}
}

// Ensure sql import is used when db is non-nil in integration scenarios.
var _ *sql.DB
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./internal/server/... -v 2>&1 | head -30
```

Expected: compilation error — `server.New` no longer accepts the new signature yet.

- [ ] **Step 3: Update `internal/server/server.go`**

Replace the content of `internal/server/server.go`:
```go
// Package server implements the HTTP server, routing, and request handlers.
package server

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"bingo/internal/config"
	"bingo/internal/paste"
)

// Server holds the HTTP router, application configuration, and storage.
type Server struct {
	mux  *http.ServeMux
	cfg  *config.Config
	db   *sql.DB
	repo paste.Repository
}

// New creates a Server with all API routes registered.
// db may be nil in tests that do not exercise the healthz DB ping.
func New(cfg *config.Config, db *sql.DB, repo paste.Repository) *Server {
	s := &Server{
		mux:  http.NewServeMux(),
		cfg:  cfg,
		db:   db,
		repo: repo,
	}
	s.registerRoutes()
	return s
}

// ServeHTTP implements http.Handler, delegating to the internal ServeMux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// registerRoutes wires all API endpoints to their handler methods.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/v1/healthz", s.handleHealthz)
	s.mux.HandleFunc("POST /api/v1/pastes", s.handleCreatePaste)
	s.mux.HandleFunc("GET /api/v1/pastes/{key}", s.handleGetPaste)
	s.mux.HandleFunc("GET /api/v1/pastes/{key}/raw", s.handleGetPasteRaw)
	s.mux.HandleFunc("DELETE /api/v1/pastes/{key}", s.handleDeletePaste)
	s.mux.HandleFunc("GET /api/v1/languages", s.handleListLanguages)
}

// writeJSON sets Content-Type: application/json, writes the status, and encodes v.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errDetail is the inner object in all error responses.
type errDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errEnvelope is the standard JSON error body.
type errEnvelope struct {
	Error errDetail `json:"error"`
}

// writeError writes a JSON error envelope with the given HTTP status.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errEnvelope{Error: errDetail{Code: code, Message: message}})
}
```

- [ ] **Step 4: Create `internal/server/handlers.go`**

Create `internal/server/handlers.go`:
```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"bingo/internal/paste"
)

// handleHealthz returns {"status":"ok"} and, when a DB is wired, pings it.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if s.db != nil {
		if err := s.db.PingContext(r.Context()); err != nil {
			slog.Error("healthz db ping failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "db_unavailable", "Database is unavailable.")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// createRequest is the JSON body for POST /api/v1/pastes.
type createRequest struct {
	Content   string `json:"content"`
	Language  string `json:"language"`
	Title     string `json:"title"`
	ExpiresIn string `json:"expires_in"`
}

// createResponse is the JSON body for a 201 Created response.
type createResponse struct {
	Key       string    `json:"key"`
	URL       string    `json:"url"`
	RawURL    string    `json:"raw_url"`
	Language  string    `json:"language"`
	Title     string    `json:"title,omitempty"`
	SizeBytes int       `json:"size_bytes"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) handleCreatePaste(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body is not valid JSON.")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "missing_content", "Content must not be empty.")
		return
	}
	if int64(len(req.Content)) > s.cfg.MaxPasteSizeBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "content_too_large",
			"Paste exceeds the configured size limit.")
		return
	}
	if req.Language == "" {
		req.Language = "plaintext"
	}
	if !paste.IsValidLanguage(req.Language) {
		writeError(w, http.StatusBadRequest, "unknown_language",
			"Language is not in the supported registry.")
		return
	}

	expiresIn := req.ExpiresIn
	if expiresIn == "" {
		expiresIn = string(paste.ExpiresIn3mo)
	}
	ei, err := paste.ParseExpiresIn(expiresIn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_expires_in",
			"expires_in must be one of: 1d, 1w, 1mo, 3mo, 1y.")
		return
	}

	params := paste.CreateParams{
		Content:   req.Content,
		Language:  req.Language,
		Title:     req.Title,
		ExpiresIn: ei,
	}
	p, err := s.repo.Create(r.Context(), params)
	if err != nil {
		slog.Error("create paste", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return
	}

	writeJSON(w, http.StatusCreated, createResponse{
		Key:       p.Key,
		URL:       s.cfg.BaseURL + "/" + p.Key,
		RawURL:    s.cfg.BaseURL + "/api/v1/pastes/" + p.Key + "/raw",
		Language:  p.Language,
		Title:     p.Title,
		SizeBytes: p.SizeBytes,
		ExpiresAt: p.ExpiresAt.UTC(),
		CreatedAt: p.CreatedAt.UTC(),
	})
}

// pasteResponse is returned by GET /api/v1/pastes/{key}.
type pasteResponse struct {
	Key       string    `json:"key"`
	URL       string    `json:"url"`
	RawURL    string    `json:"raw_url"`
	Content   string    `json:"content"`
	Language  string    `json:"language"`
	Title     string    `json:"title,omitempty"`
	SizeBytes int       `json:"size_bytes"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) handleGetPaste(w http.ResponseWriter, r *http.Request) {
	k := r.PathValue("key")
	p, err := s.getPasteOrError(w, r.Context(), k)
	if err != nil {
		return // response already written
	}
	writeJSON(w, http.StatusOK, s.toPasteResponse(p))
}

func (s *Server) handleGetPasteRaw(w http.ResponseWriter, r *http.Request) {
	k := r.PathValue("key")
	p, err := s.getPasteOrError(w, r.Context(), k)
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(p.Content))
}

// getPasteOrError fetches a paste, applies lazy expiry, and writes an error
// response when the paste is missing or expired. Returns non-nil err when
// the caller should stop processing.
func (s *Server) getPasteOrError(w http.ResponseWriter, ctx context.Context, k string) (*paste.Paste, error) {
	p, err := s.repo.GetByKey(ctx, k)
	if err != nil {
		if errors.Is(err, paste.ErrNotFound) {
			writeError(w, http.StatusNotFound, "paste_not_found", "Paste not found.")
			return nil, err
		}
		slog.Error("get paste", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return nil, err
	}
	// Lazy expiry: delete and return 404 if the paste has expired.
	if time.Now().After(p.ExpiresAt) {
		_ = s.repo.Delete(ctx, k)
		writeError(w, http.StatusNotFound, "paste_not_found", "Paste not found.")
		return nil, errors.New("expired")
	}
	return p, nil
}

func (s *Server) handleDeletePaste(w http.ResponseWriter, r *http.Request) {
	k := r.PathValue("key")
	// Phase 2: authentication not yet implemented; anonymous pastes only.
	// Phase 3 will enforce owner_id checks.
	if err := s.repo.Delete(r.Context(), k); err != nil {
		slog.Error("delete paste", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListLanguages(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]string{"languages": paste.AllLanguages()})
}

func (s *Server) toPasteResponse(p *paste.Paste) pasteResponse {
	return pasteResponse{
		Key:       p.Key,
		URL:       s.cfg.BaseURL + "/" + p.Key,
		RawURL:    s.cfg.BaseURL + "/api/v1/pastes/" + p.Key + "/raw",
		Content:   p.Content,
		Language:  p.Language,
		Title:     p.Title,
		SizeBytes: p.SizeBytes,
		ExpiresAt: p.ExpiresAt.UTC(),
		CreatedAt: p.CreatedAt.UTC(),
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./internal/server/... -v
```

Expected: all tests PASS (12+ passing).

- [ ] **Step 6: Run full suite**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
git add internal/server/
git commit -m "feat(server): wire handlers to paste.Repository with full input validation

- Server.New() now accepts *sql.DB and paste.Repository
- handlers.go: all 6 handlers with real logic
  - POST /api/v1/pastes: size/language/expires_in validation, 201 with full JSON
  - GET /api/v1/pastes/{key}: lazy expiry (delete + 404 on expired)
  - GET /api/v1/pastes/{key}/raw: text/plain + X-Content-Type-Options: nosniff
  - DELETE /api/v1/pastes/{key}: 204 No Content (Phase 3 adds owner check)
  - GET /api/v1/languages: sorted language list
  - GET /api/v1/healthz: DB ping when db != nil
- Error codes: missing_content, invalid_expires_in, unknown_language, content_too_large
- Default language plaintext, default expires_in 3mo
- Tests: 12 cases covering success, validation errors, lazy expiry, not-found

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 7: Main Wiring Update

**Files:**
- Modify: `cmd/bingo/main.go` (wire DB open → migrate → repo → server → sweep)

**Interfaces:**
- Consumes:
  - `bingo/internal/database.Open(databaseURL string) (*sql.DB, error)`
  - `bingo/internal/database.Migrate(db *sql.DB) error`
  - `bingo/internal/paste.NewPostgresRepository(db *sql.DB) *PostgresRepository`
  - `bingo/internal/paste.StartSweep(ctx, repo, interval) func()`
  - `bingo/internal/server.New(cfg, db, repo) *Server` (updated signature)

- [ ] **Step 1: Update `cmd/bingo/main.go`**

Replace the `run()` function and imports:
```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bingo/internal/config"
	"bingo/internal/database"
	"bingo/internal/paste"
	"bingo/internal/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	slog.Info("database migrations applied")

	repo := paste.NewPostgresRepository(db)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	stopSweep := paste.StartSweep(ctx, repo, time.Hour)
	defer stopSweep()

	srv := server.New(cfg, db, repo)

	httpSrv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("server starting", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go build ./...
```

Expected: exit 0, no errors.

- [ ] **Step 3: Run full test suite**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
go test ./...
```

Expected: all PASS.

- [ ] **Step 4: Run golangci-lint**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
golangci-lint run ./...
```

Expected: exit 0, no issues.

- [ ] **Step 5: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase2
git add cmd/bingo/main.go
git commit -m "feat(cmd): wire Phase 2 storage into main entrypoint

- database.Open() + database.Migrate() on startup
- paste.NewPostgresRepository(db) passed to server.New()
- paste.StartSweep(ctx, repo, 1h) started before HTTP server
- Shutdown sequence: signal → sweep cancel → HTTP drain → DB close

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Phase 2 Completion Checklist

- [ ] `go build ./...` exits 0
- [ ] `go test ./...` exits 0 (all packages PASS)
- [ ] `golangci-lint run ./...` exits 0
- [ ] `gofmt -d .` produces zero diff
- [ ] `internal/key`: GenerateKey produces correct length, base62 charset, no duplicates in 1000 samples
- [ ] `internal/paste`: ParseExpiresIn rejects invalid values; language registry accepts known keys; ErrNotFound defined
- [ ] `internal/paste/postgres.go`: compiles and satisfies Repository interface
- [ ] `internal/paste/sweep.go`: StartSweep goroutine calls DeleteExpired on tick, stops on cancel
- [ ] `internal/server`: all handlers return correct status codes with correct error codes
- [ ] `cmd/bingo/main.go`: wires DB + repo + server + sweep
- [ ] 7 commits on `feat/bingo-phase2` (key, paste-types, database, repo, sweep, handlers, main)
- [ ] No TODO/TBD/placeholder text in any committed `.go` file
