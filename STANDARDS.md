# STANDARDS.md: bingo Engineering Standards

The single reference for **how** the bingo Go workload is built: development process,
code style, testing, logging, database access, and CI. These standards are
**enforced** during execution. Where this document and a linked external guide
conflict, the rule stated here wins (it reflects a deliberate choice for bingo).

Cross-references: [MONOREPO.md](MONOREPO.md) (repo layout & CI plumbing),
[PLAN.md](PLAN.md) (scope & phases), [SCHEMA.md](SCHEMA.md) (DB schema & API),
[WORKFLOW.md](WORKFLOW.md) (UX flow).

---

## 1. Development Process: Test-Driven Development (TDD)

**TDD is mandatory.** Every feature and bugfix follows the red → green → refactor loop:

1. **Write the test first.** Capture the desired behaviour as a test.
2. **Run it and watch it fail.** A test that has never failed proves nothing. Confirm
   it fails for the *expected* reason (assertion, not a compile error you forgot).
3. **Write the minimum implementation** to make the test pass.
4. **Run the tests** and confirm green.
5. **Refactor** with the tests as a safety net, keeping them green.

- The **`test-driven-development` skill** from
  [Superpowers](https://github.com/obra/Superpowers) is used to drive this loop during
  execution — follow it exactly; do not adapt away the discipline.
- Never write implementation code before a failing test exists for it.
- Never claim work is complete without running the tests and showing they pass.

---

## 2. Dependencies: Follow Canonical Go Projects

Model bingo's dependency choices on Canonical's production Go applications. The primary
reference is **Pebble** ([canonical/pebble](https://github.com/canonical/pebble)); also
observe **Juju** ([juju/juju](https://github.com/juju/juju)) and
**LXD** ([canonical/lxd](https://github.com/canonical/lxd)).

**Pebble's stack (from its `go.mod`)** — use as the reference baseline:

| Dependency | Purpose |
| --- | --- |
| `github.com/gorilla/mux` | HTTP routing |
| `github.com/gorilla/websocket` | WebSockets |
| `github.com/canonical/go-flags` | CLI flag parsing |
| `github.com/canonical/x-go` | Shared Canonical Go utilities |
| `golang.org/x/sys`, `golang.org/x/term` | Low-level system / terminal |
| `gopkg.in/yaml.v3` | YAML config |

**Guidance for bingo specifically (these choices override the baseline where noted):**

- **Standard library first.** Reach for `net/http`, `database/sql`, `log/slog`,
  `context`, `testing` before adding a dependency.
- **Routing:** `net/http` (Go 1.22+ enhanced `ServeMux`) or `chi`; `gorilla/mux` is
  acceptable as it matches the Canonical baseline (PLAN.md §3).
- **Prefer Canonical-maintained libraries** (e.g. `canonical/x-go`,
  [`canonical/sqlair`](https://github.com/canonical/sqlair)) over third-party
  equivalents when a real need arises.
- **Avoid heavy frameworks and ORMs.** Keep dependencies few, maintained, and
  justifiable. Pin versions in `go.mod`/`go.sum`.

> Note on testing libraries: Pebble and Juju use `gopkg.in/check.v1` (gocheck). For
> **bingo we deliberately use the standard `testing` package with no assertion
> library** (see §4) — this aligns with the Google Go Style Guide, our primary style
> reference.

---

## 3. Code Style (Enforced)

### Primary reference — Google Go Style Guide

bingo's primary, normative style reference is Google's Go Style Guide:

- **Overview:** <https://google.github.io/styleguide/go/>
- **Style Guide (normative):** <https://google.github.io/styleguide/go/guide>
- **Decisions (detailed rulings):** <https://google.github.io/styleguide/go/decisions>
- **Best Practices:** <https://google.github.io/styleguide/go/best-practices>
- **Effective Go:** <https://go.dev/doc/effective_go>

### Canonical conventions to observe

Canonical has no published Go style guide; follow the conventions visible in their
major projects:

- **Juju** (<https://github.com/juju/juju>) — conventional commits, signed commits, CLA.
- **LXD** (<https://github.com/canonical/lxd>) — enforces `gofmt` via `make update-fmt`,
  GPG-signed commits, DCO sign-off.

**Adopt these for bingo:**

- **`gofmt`-clean** at all times (CI enforces it; no unformatted code merges).
- **Conventional commit** messages.
- **Signed commits** and **DCO sign-off** (`Signed-off-by:` trailer / `git commit -s`).

### Project layout

Follow the [Standard Go project layout](https://github.com/golang-standards/project-layout)
as already specified in [MONOREPO.md](MONOREPO.md): thin entry points under `cmd/`, all
business logic under `internal/` (domain packages: `server/`, `database/`, `paste/`, …).

---

## 4. Testing (Enforced)

### References

- **Google Go Testing Best Practices:**
  <https://google.github.io/styleguide/go/best-practices#tests>
- **Effective Go:** <https://go.dev/doc/effective_go>

### Unit tests

- Use the **standard `testing` package** — no third-party test/assertion frameworks.
- **Table-driven tests** are the default pattern for varied inputs.
- Mark helpers with **`t.Helper()`** so failures report the caller's line.
- **No assertion libraries.** Use plain comparisons and `t.Errorf` / `t.Fatalf`:

  ```go
  got := Slugify(input)
  if got != want {
      t.Errorf("Slugify(%q) = %q, want %q", input, got, want)
  }
  ```

- Prefer `cmp.Diff` (`github.com/google/go-cmp`) for comparing structs/slices in
  error messages where helpful; still gate with a plain `if`.

### Integration tests

- Use **`TestMain`** for shared setup/teardown (e.g. spin up / migrate / tear down a
  real PostgreSQL database).
- Use **real transports, not mocks**, per Google style — exercise the real HTTP server
  and a real database, not hand-rolled stand-ins, at the integration boundary.
- Gate integration tests that need external services behind build tags or
  `testing.Short()` so `go test -short ./...` stays fast and hermetic.

### Coverage

- Measure with `go test -coverprofile=cover.out ./...`.
- **Gate: ≥ 85% coverage on internal Go packages** (matches [MONOREPO.md](MONOREPO.md)).
- Aim for **meaningful coverage, not 100%** — cover behaviour and edge cases, not
  trivial getters.

### Linting

Run **`golangci-lint`** with at least these linters enabled:

- `gofmt` — formatting
- `govet` — suspicious constructs
- `staticcheck` — correctness & simplifications
- `sloglint` — correct `log/slog` usage

Keep cyclomatic complexity **< 10 per function** (MONOREPO.md gate).

---

## 5. Logging (`log/slog`, structured)

Use the standard library **`log/slog`** (structured logging, Go 1.21+). Logs stream
unbuffered to `stdout`/`stderr` for 12-factor compliance (PLAN.md §Phase 4).

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
```

### Levels (with bingo examples)

| Level | Use for | bingo examples |
| --- | --- | --- |
| `DEBUG` | Diagnostic detail | Request parsing, template rendering, DB query details |
| `INFO`  | Normal lifecycle events | Paste created, paste viewed, server started, config loaded |
| `WARN`  | Recoverable / approaching limits | Rate limit approaching, paste near expiry threshold |
| `ERROR` | Failures needing attention | DB connection failure, template render error, paste not found on expected path |

### Rules

- **Structured attributes, never string interpolation:**
  `slog.String("paste_id", id)` — not `fmt.Sprintf("paste %s", id)`.
- **Log at request boundaries** (middleware) with `request_id`, `method`, `path`,
  `status`, and `duration`.
- **Never log paste content** or other PII / sensitive data (delete tokens, raw bodies).
- **Keep logging minimal and purposeful** — log things that are *actionable*. Go
  convention favours signal over noise.

---

## 6. Database (Go + PostgreSQL)

### Architecture pattern

```
internal/
├── database/            # DB connection, migrations, shared helpers
│   ├── database.go      # Open, Ping, pool config
│   └── migrations/      # SQL migration files
├── paste/               # Domain package (entity + repository)
│   ├── paste.go         # Entity struct (plain Go struct, not DB-tagged)
│   ├── repository.go    # Interface: Create, GetByID, List, Delete
│   └── postgres.go      # Concrete implementation using database/sql
```

### Key principles

- **`database/sql`, not an ORM** — explicit, debuggable SQL.
- **Driver: pgx** via `github.com/jackc/pgx/v5/stdlib` (the modern, maintained
  PostgreSQL driver — **not** the deprecated `lib/pq`).
- **Repository pattern** — define interfaces for data access; implementations hold a
  `*sql.DB`. Makes testing with fakes trivial.
- **Entities are plain structs** — no ORM tags; `Scan` into fields explicitly.
- **Always use `Context` variants** — `QueryContext`, `ExecContext`, `QueryRowContext`
  — to enable timeouts and cancellation.
- **Parameterized queries only** — `$1`, `$2` placeholders. **Never** `fmt.Sprintf`
  into SQL (SQL-injection risk).
- **Configure the connection pool explicitly** — `SetMaxOpenConns`, `SetMaxIdleConns`,
  `SetConnMaxLifetime`.
- **Migrations** — [`golang-migrate`](https://github.com/golang-migrate/migrate) with
  versioned `.sql` files in the repo; run at startup or as a separate step.
- **Transactions** — `db.BeginTx(ctx, nil)` with a deferred rollback.
- **Nullable columns** — `sql.NullString` etc., or pointer types (`*string`).
- **Always `defer rows.Close()`** — even when iterating to completion.

### References

- Go database tutorial: <https://go.dev/doc/tutorial/database-access>
- Go database docs (full series): <https://go.dev/doc/database/>
- pgx driver: <https://github.com/jackc/pgx>
- golang-migrate: <https://github.com/golang-migrate/migrate>
- SQLair (Canonical, optional): <https://github.com/canonical/sqlair>

### Example: repository interface

```go
type PasteRepository interface {
    Create(ctx context.Context, paste *Paste) error
    GetByID(ctx context.Context, id string) (*Paste, error)
    List(ctx context.Context, limit, offset int) ([]*Paste, error)
    Delete(ctx context.Context, id string) error
}
```

### Example: transaction pattern

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback() // no-op if already committed

// ... do work with tx ...

return tx.Commit()
```

---

## 7. Continuous Integration (charm-ci)

CI uses Canonical's **`charm-ci`** reusable workflows. **Do NOT use
`operator-workflows`** — use `charm-ci` exclusively. charm-ci is language-agnostic: the
Go workload is fine because the 12-factor charm layer is always a Python operator.

| Workflow | Responsibility |
| --- | --- |
| `internal_tests.yaml` | Go unit tests — `go test`, coverage **≥ 85%**, `golangci-lint` |
| `charms_lint_and_unit.yaml` | Charm lint/unit tests via `tox` |
| `charms_integration.yaml` | Delegates to charm-ci `integration-test.yml` |
| `publish_charms.yml` | Delegates to charm-ci `publish-artifacts.yml` |

- Build manifest (`artifacts.yaml`), test orchestration (`spread.yaml`), and env
  provisioning (`concierge.yaml`) follow the **`github-runner-operators`** pattern.
- See [MONOREPO.md](MONOREPO.md) for the full CI plumbing and what to copy.

---

## Quick checklist (per change)

- [ ] Failing test written first, confirmed red for the right reason.
- [ ] Minimal implementation makes it green.
- [ ] `gofmt`-clean; `golangci-lint` (`gofmt`, `govet`, `staticcheck`, `sloglint`) passes.
- [ ] Standard `testing`, table-driven, `t.Helper()`, no assertion libs.
- [ ] Coverage ≥ 85% on internal packages; complexity < 10/function.
- [ ] `log/slog` structured attributes; no PII/paste content logged.
- [ ] DB: context variants, parameterized queries, `defer rows.Close()`.
- [ ] Conventional commit, signed, DCO sign-off.
