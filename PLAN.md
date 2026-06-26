# PLAN.md: Canonical Pastebin (bingo) — Go Re-implementation

## 1. Project Context & Vision

The goal of this project is to replace Canonical's outdated, legacy Pastebin
application with a modern, maintainable, 12-Factor compliant workload named
**bingo**.

The old Pastebin applications (`pastebin.canonical.com` and `paste.ubuntu.com`) will
be decommissioned with Prodstack 5. **bingo** will launch on **Prodstack 7** as a
greenfield deployment — users are expected to migrate themselves; there is no
automated data migration or backwards-compatible URL routing.

The Python/Django-based `dpaste` project[^1] is used as a **conceptual reference**
for this re-implementation. `dpaste` is a simple ~500-line Django application and the
agent should have no difficulty producing an equivalent Go version informed by its
business logic and feature set. The `dpaste` codebase will be included in the
workspace context for reference.

We are building this from scratch using **Go (Golang)**, exposing a strict **JSON
API**, with a **React + TypeScript** frontend styled via Canonical's Pragma[^2]
component library.

[^1]: dpaste — <https://github.com/DarrenOfficial/dpaste> (conceptual reference)
[^2]: Pragma — Canonical's component & styling MCP server: <https://github.com/canonical/pragma>

---

## 2. Repository Structure: Mono-repo

This project uses a **single mono-repo** (`bingo-k8s-operator`) containing both the
Go application workload and the Canonical 12-Factor OCI charm, modeled on the
`github-runner-operators`[^3] repository pattern.

[^3]: github-runner-operators — the reference mono-repo: <https://github.com/canonical/github-runner-operators>

### Target Layout

```
bingo-k8s-operator/
├── artifacts.yaml              # charm-ci build manifest (1 rock + 1 charm)
├── spread.yaml                 # charm-ci test orchestration
├── concierge.yaml              # env provisioning (Juju, MicroK8s, LXD)
├── bingo-rockcraft.yaml        # OCI rock definition
│
├── go.mod / go.sum
├── cmd/
│   └── bingo/
│       └── main.go             # thin entry point
├── internal/                   # all Go business logic (private to module)
│   ├── server/                 # HTTP server, middleware, routing
│   ├── paste/                  # domain: entity, repository interface, postgres impl
│   ├── database/               # connection pool, migrations, helpers
│   ├── slug/                   # base62 slug generation
│   └── auth/                   # OIDC middleware
│
├── web/                        # React + TypeScript frontend (Pragma components)
│   ├── package.json
│   ├── tsconfig.json
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/
│   │   ├── pages/
│   │   └── utils/
│   └── tests/                  # frontend tests (unit + e2e)
│
├── charmcraft.yaml             # single charm (go-framework extension)
├── src/
│   └── charm.py                # BingoCharm(paas_charm.go.Charm)
├── tests/
│   ├── unit/
│   │   └── test_charm.py
│   └── integration/
│       └── test_charm.py
│
├── .github/workflows/
│   ├── internal_tests.yaml     # Go: go test ./... + coverage gate
│   ├── frontend_tests.yaml     # React: lint, unit, e2e
│   ├── charms_lint_and_unit.yaml
│   ├── charms_integration.yaml
│   └── publish_charms.yml
│
├── PLAN.md
└── .env.example
```

The Go workload follows the community project layout[^4]: thin entry points under
`cmd/`, all business logic under `internal/`. The charm is a thin Python layer using
`paas_charm.go.Charm` — the `go-framework` extension owns reconciliation.

[^4]: Go community project layout — <https://github.com/golang-standards/project-layout>

---

## 3. Technical Stack & Architecture

### Backend

| Component | Choice | Notes |
|-----------|--------|-------|
| Language | Go 1.22+ | Standard library first |
| Routing | `net/http` (enhanced `ServeMux`) | Go 1.22 method-based routing; no framework |
| Database | PostgreSQL | Attached resource, stateless app |
| DB Driver | `pgx/v5`[^5] via `database/sql` | Modern, maintained; **not** `lib/pq` |
| Migrations | `golang-migrate`[^6] | Versioned `.sql` files |
| Logging | `log/slog` (structured, JSON) | Streams to `stdout`/`stderr` (12-factor) |
| Auth | OIDC via Identity Platform (CIdP) | Associate pastes with authenticated users |
| Config | Environment variables exclusively | 12-factor; `.env.example` documents all vars |

[^5]: pgx — <https://github.com/jackc/pgx>
[^6]: golang-migrate — <https://github.com/golang-migrate/migrate>

### Frontend

| Component | Choice | Notes |
|-----------|--------|-------|
| Framework | React + TypeScript | Canonical's standard for interactive web apps |
| Components & Styles | Pragma[^2] MCP components (`@canonical/react-components`) | React-native; used by MAAS UI, Juju Dashboard, snapcraft.io, ubuntu.com/pro |
| Syntax Highlighting | Client-side (React library, e.g. `react-syntax-highlighter`) | API transfers language metadata only |
| Build | Vite (or CRA replacement) | Fast dev server, optimized production builds |
| Testing | Vitest (unit) + Playwright (e2e) | Full frontend coverage |

**Frontend workflow:** React renders the UI (paste form, paste viewer, syntax
highlighting) client-side. It fetches data from the Go JSON API via HTTP. Pragma[^2]
components plug in directly as React components.

### Deployment & CI

| Component | Choice | Notes |
|-----------|--------|-------|
| Packaging | OCI rock (`bingo-rockcraft.yaml`) | Built from `cmd/bingo` |
| Charm | `charmcraft.yaml` (go-framework) | Wraps the rock as `app-image` resource |
| CI System | `charm-ci`[^7] (NOT `operator-workflows`) | `artifacts.yaml` + `spread.yaml` + `concierge.yaml` |
| Go CI | `go test`, `golangci-lint`, coverage ≥ 85% | `internal_tests.yaml` |
| Frontend CI | lint, unit tests, e2e tests | `frontend_tests.yaml` |

[^7]: charm-ci — <https://github.com/canonical/charm-ci>

---

## 4. Authentication: OIDC via Identity Platform

Authentication uses **OIDC** (OpenID Connect) connected to the new Canonical Identity
Platform (CIdP). **Authentication is required to access the application** — there
are no anonymous pastes.

### Purpose

All users must authenticate via OIDC before using bingo. Every paste is associated
with the authenticated user's identity (`owner_id` is always populated).
Authenticated users gain:

- A "my pastes" view listing their own pastes.
- All pastes are owned and attributed to the creating user.

### Security Considerations

- **Token validation:** All OIDC tokens must be validated server-side (signature,
  issuer, audience, expiry) before trusting claims.
- **Session management:** Use secure, HttpOnly, SameSite=Strict cookies for session
  state. Never expose tokens to JavaScript.
- **CSRF protection:** All state-changing API requests from the frontend must include
  CSRF tokens.
- **CORS:** Restrict allowed origins to the production domain(s). Do not use wildcard
  origins in production.
- **Authorization:** Only the paste owner can delete their own paste. Verify
  ownership server-side on every privileged action. Unauthenticated requests are
  rejected with `401`.

### Implementation

- OIDC middleware in `internal/auth/` handles the authorization code flow callback,
  token refresh, and session binding.
- The Go API reads user identity from the validated session and injects it into the
  request context.
- Configuration via environment variables: `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`,
  `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL`.

---

## 5. Feature Requirements

Referencing `dpaste`[^1] as the conceptual baseline:

### Core Features

- **Paste creation & retrieval:** Submit raw text with syntax highlighting metadata
  (language key), return a unique slug via JSON, fetch paste content + metadata via
  JSON API.
- **Unique slug generation:** Base62, collision-resistant, starting at 4 chars; on
  `UNIQUE` violation, retry with length + 1 (dpaste pattern).
- **Expiration (mandatory):** Every paste expires. Allowed durations: `1d`, `1w`,
  `1mo`, `3mo` (default), `1y` (max). No keep-forever option.
- **Burn-after-read:** One-time paste semantics with configurable `view_limit`.
  Atomic read-then-burn ensures concurrent readers cannot both win.
- **Deletion:** Only the paste owner (authenticated user who created it) can delete
  their paste. Ownership is enforced server-side via `owner_id`.
- **Raw endpoint:** `GET /api/v1/pastes/{slug}/raw` returns `text/plain` (curl-friendly).
- **Language registry:** `GET /api/v1/languages` serves available syntax languages,
  validated on create.
- **Background expiry sweep:** `DELETE FROM pastes WHERE expires_at < now();` plus
  lazy expiry on `GET` (expired → `404` + delete).
- **Rate limiting:** Per-IP token bucket at the API gateway level (out of app scope
  for MVP; app returns `429` errors from the gateway).

### Content Size Limit

Maximum paste content: **5 MiB** (5,242,880 bytes). Enforced at the API boundary
(`413 Content Too Large`) and via database CHECK constraint.

---

## 6. Database Schema (PostgreSQL)

```sql
CREATE TABLE pastes (
    id                BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug              TEXT        NOT NULL UNIQUE,
    content           TEXT        NOT NULL,
    language          VARCHAR(64) NOT NULL DEFAULT 'plaintext',
    title             VARCHAR(255),
    size_bytes        INTEGER     NOT NULL,
    burn_after_read   BOOLEAN     NOT NULL DEFAULT false,
    expires_at        TIMESTAMPTZ NOT NULL,
    view_count        INTEGER     NOT NULL DEFAULT 0,
    view_limit        INTEGER,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    owner_id          BIGINT      NOT NULL,  -- always populated; auth is required

    CONSTRAINT view_limit_consistent CHECK (
        (burn_after_read     AND view_limit IS NOT NULL AND view_limit >= 1) OR
        (NOT burn_after_read AND view_limit IS NULL)
    ),
    CONSTRAINT view_count_nonneg     CHECK (view_count >= 0),
    CONSTRAINT size_within_limit     CHECK (size_bytes BETWEEN 1 AND 5242880),
    CONSTRAINT slug_length           CHECK (char_length(slug) BETWEEN 4 AND 32),
    CONSTRAINT expiry_after_creation CHECK (expires_at > created_at)
);

CREATE INDEX pastes_expires_at_idx ON pastes (expires_at);
CREATE INDEX pastes_owner_id_idx   ON pastes (owner_id);
```

### Database Access Principles

- **`database/sql`, not an ORM** — explicit, debuggable SQL.
- **Repository pattern** — interfaces for data access; implementations hold `*sql.DB`.
- **Entities are plain structs** — no ORM tags; `Scan` into fields explicitly.
- **Always use `Context` variants** — `QueryContext`, `ExecContext`, `QueryRowContext`.
- **Parameterized queries only** — `$1`, `$2` placeholders. **Never** string
  interpolation into SQL.
- **Configure connection pool** — `SetMaxOpenConns`, `SetMaxIdleConns`,
  `SetConnMaxLifetime`.
- **Transactions** — `db.BeginTx(ctx, nil)` with deferred rollback.
- **Always `defer rows.Close()`**.

---

## 7. JSON API (`/api/v1`)

All requests/responses are `application/json` (except `/raw`). Timestamps are RFC 3339
UTC. All endpoints (except `/api/v1/healthz`) require a valid OIDC session.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/pastes` | Create a paste |
| `GET` | `/api/v1/pastes/{slug}` | Retrieve paste content + metadata |
| `GET` | `/api/v1/pastes/{slug}/raw` | Raw text/plain body |
| `DELETE` | `/api/v1/pastes/{slug}` | Delete (owner only) |
| `GET` | `/api/v1/languages` | Available syntax languages |
| `GET` | `/api/v1/healthz` | Liveness + DB ping |

### Create Request (`POST /api/v1/pastes`)

```json
{
  "content": "print('hello')",
  "language": "python",
  "title": "demo snippet",
  "expires_in": "3mo",
  "burn_after_read": false
}
```

### Create Response (`201 Created`)

```json
{
  "slug": "aB3xY",
  "url": "https://paste.canonical.com/aB3xY",
  "raw_url": "https://paste.canonical.com/api/v1/pastes/aB3xY/raw",
  "language": "python",
  "title": "demo snippet",
  "size_bytes": 14,
  "burn_after_read": false,
  "expires_at": "2026-09-21T12:00:00Z",
  "created_at": "2026-06-23T12:00:00Z"
}
```

### Error Envelope (all 4xx/5xx)

```json
{ "error": { "code": "content_too_large", "message": "Paste exceeds the 5 MiB limit." } }
```

| Status | Codes |
|--------|-------|
| 400 | `invalid_request`, `missing_content`, `invalid_expires_in`, `unknown_language` |
| 401 | `unauthenticated` |
| 403 | `forbidden` (not the paste owner) |
| 404 | `paste_not_found` (also expired/burned) |
| 413 | `content_too_large` |
| 429 | `rate_limited` |
| 500 | `internal_error` |

---

## 8. Security: Input/Output Hardening

Because paste content is user-supplied and rendered in the browser, rigorous hardening
is critical at every layer:

### Backend (Go API)

- **Input validation at the boundary:** reject content > 5 MiB, validate `language`
  against the registry, enforce `expires_in` enum, sanitize `title` length.
- **Content scanning:** check submitted content for known exploit patterns (embedded
  `<script>` tags, encoded payloads) before storage. Log and reject suspicious input.
- **Output encoding:** JSON responses must properly escape all user-supplied strings.
  The `encoding/json` package handles this by default — never use raw string
  concatenation for JSON construction.
- **Content-Type headers:** Always set explicit `Content-Type` headers. The `/raw`
  endpoint must return `text/plain; charset=utf-8` with
  `X-Content-Type-Options: nosniff` to prevent browser MIME-sniffing.
- **Security headers:** All responses include `X-Frame-Options: DENY`,
  `X-Content-Type-Options: nosniff`, `Content-Security-Policy` with strict directives.

### Frontend (React)

- **Never use `dangerouslySetInnerHTML`** with untreated API responses.
- **Sanitize before rendering:** even though React escapes JSX by default, explicitly
  sanitize any content passed to syntax highlighting libraries or rendered outside
  normal JSX text interpolation.
- **CSP enforcement:** The frontend build must support strict Content-Security-Policy
  headers (no `unsafe-inline` for scripts).
- **Validate API responses:** Check response shapes and types before rendering. Do not
  trust that the API response is well-formed — defensive parsing protects against
  compromised backends or MITM.

---

## 9. URL Schema & Routing

### Production URL

- **`https://paste.canonical.com/{slug}`** — canonical URL for viewing a paste.
- **`https://paste.canonical.com/`** — default page is the "new paste" form.

### API Routing

- `POST /api/v1/pastes` — create.
- `GET /api/v1/pastes/{slug}` — retrieve.
- `GET /api/v1/pastes/{slug}/raw` — raw text.
- `DELETE /api/v1/pastes/{slug}` — delete.
- `GET /api/v1/languages` — language list.
- `GET /api/v1/healthz` — health check.

**Note:** There is no legacy URL routing. The old `pastebin.canonical.com/p/{id}/` and
`paste.ubuntu.com/p/{id}/` paths are **not supported**. The Ubuntu pastebin will be
decommissioned. Users migrate themselves to the new service.

---

## 10. User Workflow

All users must authenticate via OIDC before accessing any page. Unauthenticated
requests redirect to the OIDC login flow.

1. **Login** — user authenticates via OIDC (CIdP). On success, session is established.

2. **Default page** (`/`) is the "new paste" form.
   - Fields: Title (optional), Syntax (from `/api/v1/languages`), Expiration, Content.
   - Optional: Burn-after-read (+ view limit).

3. **After creation**, redirect to `/{slug}` showing the paste content.
   - View page shows: creation date, expiry, syntax type, content with client-side
     syntax highlighting, "view raw" link, toggle wrap, copy to clipboard.
   - For burn pastes, show `remaining_views`.

4. **Direct navigation** to `/{slug}` shows the view page.
   - Expired / missing / burned-out → `404` state.

5. **"New paste" link** visible when viewing an existing paste.

6. **"My pastes"** view lists the authenticated user's own pastes.

---

## 11. Development Process: Test-Driven Development

**TDD is mandatory** for all Go backend and React frontend work. The
**`test-driven-development` skill** from the Superpowers framework[^8] drives the
red → green → refactor loop during agent execution.

[^8]: Superpowers skills framework — <https://github.com/obra/Superpowers>

### TDD Loop (enforced)

1. **Write the test first.** Capture desired behaviour.
2. **Run it and watch it fail.** Confirm it fails for the *expected* reason.
3. **Write the minimum implementation** to make the test pass.
4. **Run tests** — confirm green.
5. **Refactor** with tests as safety net.

### Backend Testing (Go)

- **Standard `testing` package** — no third-party assertion frameworks.
- **Table-driven tests** as the default pattern.
- **`t.Helper()`** on all test helpers.
- **Integration tests** use real PostgreSQL (via `TestMain` setup/teardown) and real
  HTTP transports — not mocks.
- **Coverage gate: ≥ 85%** on `internal/` packages.
- **Cyclomatic complexity: < 10** per function.
- **Linting:** `golangci-lint` with `gofmt`, `govet`, `staticcheck`, `sloglint`.

### Frontend Testing (React)

- **Unit tests** (Vitest): component rendering, utility functions, hook behaviour.
- **End-to-end tests** (Playwright): full user workflows — create paste, view paste,
  burn-after-read, delete with token, syntax highlighting rendering.
- **Coverage gate:** enforce meaningful coverage on components and utilities.

### Key Principle

Never write implementation code before a failing test exists for it. Never claim work
is complete without running tests and showing they pass. Use the
**`verification-before-completion` skill** from Superpowers[^8] to confirm all
assertions before declaring done.

---

## 12. Code Style & Standards

### Go Style

- **Primary reference:** Google Go Style Guide[^9].
- **`gofmt`-clean** at all times.
- **Conventional commit** messages.
- **Standard library first** — reach for `net/http`, `database/sql`, `log/slog`,
  `context`, `testing` before adding dependencies.
- **Structured logging** via `log/slog` with JSON output; never log paste content or
  PII.
- **Parameterized queries only** — never interpolate into SQL.

[^9]: Google Go Style Guide — <https://google.github.io/styleguide/go/>

### Go Dependencies (modeled on Canonical projects)

Follow the dependency patterns of Canonical's production Go applications, primarily
Pebble[^10], Juju, and LXD:

- Standard library first.
- `pgx/v5`[^5] for PostgreSQL.
- `golang-migrate`[^6] for migrations.
- `github.com/google/go-cmp` for test struct comparison.
- Avoid heavy frameworks and ORMs.

[^10]: Pebble — <https://github.com/canonical/pebble>

### React/TypeScript Style

- Follow Canonical's web team conventions (as used in MAAS UI, Juju Dashboard).
- Strict TypeScript (`strict: true`).
- Pragma[^2] components for all UI elements.
- ESLint + Prettier for formatting.

---

## 13. 12-Factor Configuration

All configuration is read exclusively from environment variables. The application
must be fully stateless.

### Required Environment Variables

| Variable | Purpose | Example |
|----------|---------|---------|
| `PORT` | HTTP server bind port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@host:5432/bingo` |
| `OIDC_ISSUER_URL` | OIDC provider URL (CIdP) | `https://identity.canonical.com` |
| `OIDC_CLIENT_ID` | OIDC client identifier | `bingo-prod` |
| `OIDC_CLIENT_SECRET` | OIDC client secret | (secret) |
| `OIDC_REDIRECT_URL` | OAuth callback URL | `https://paste.canonical.com/auth/callback` |
| `SESSION_SECRET` | Cookie encryption key | (secret) |
| `BASE_URL` | Public base URL for generated links | `https://paste.canonical.com` |
| `LOG_LEVEL` | Logging level | `info` |

An `.env.example` file documents all variables the `12factor-charm` will map to Juju
config options.

---

## 14. CI/CD: charm-ci

CI uses Canonical's **`charm-ci`**[^7] reusable workflows — **NOT**
`operator-workflows`. Driven by three declarative files:

| File | Purpose |
|------|---------|
| `artifacts.yaml` | Build manifest: 1 rock + 1 charm + resource binding |
| `spread.yaml` | Test orchestration: integration suite |
| `concierge.yaml` | Environment provisioning: Juju, MicroK8s, LXD |

### Workflows

| Workflow | Responsibility |
|----------|---------------|
| `internal_tests.yaml` | `go test ./...`, coverage ≥ 85%, `golangci-lint` |
| `frontend_tests.yaml` | ESLint, Vitest (unit), Playwright (e2e) |
| `charms_lint_and_unit.yaml` | Charm tox: fmt, lint, complexity, static, unit |
| `charms_integration.yaml` | Delegates to charm-ci `integration-test.yml` |
| `publish_charms.yml` | Delegates to charm-ci `publish-artifacts.yml` |

### `artifacts.yaml`

```yaml
version: 1
rocks:
  - name: bingo
    rockcraft-yaml: bingo-rockcraft.yaml
    platforms:
      - arch: amd64
charms:
  - name: bingo
    charmcraft-yaml: charmcraft.yaml
    channel: latest/edge
    resources:
      app-image:
        type: oci-image
        rock: bingo
    platforms:
      - arch: amd64
snaps: []
```

---

## 15. Execution Phases

Execute implementation sequentially. Complete one phase before the next. Use the
`test-driven-development` skill from Superpowers[^8] throughout — every feature
starts with a failing test.

### Phase 1: Backend Initialization & API Skeleton

- Initialize Go module (`go mod init bingo-k8s-operator`).
- Scaffold project structure: `cmd/bingo/main.go`, `internal/server/`,
  `internal/paste/`, `internal/database/`, `internal/slug/`, `internal/auth/`.
- Implement HTTP server binding to `$PORT` using `net/http` ServeMux.
- Define all API routes (§7) with stub handlers returning `501 Not Implemented`.
- Write integration test proving the server starts and `/api/v1/healthz` returns 200.
- Set up `golangci-lint` configuration.

### Phase 2: Core Logic & Storage

- Write failing tests for slug generation (base62, collision retry).
- Implement slug generation in `internal/slug/`.
- Write failing tests for paste CRUD (create, get by slug, delete, expiry).
- Implement PostgreSQL storage in `internal/paste/postgres.go` with repository
  interface.
- Implement database migrations (`internal/database/migrations/`).
- Implement burn-after-read transactional logic.
- Implement background expiry sweep (goroutine with ticker).
- Implement boundary validation (size, language, expires_in).
- Wire handlers to real storage; integration tests against real PostgreSQL.

### Phase 3: Authentication (OIDC)

- Implement OIDC middleware in `internal/auth/`.
- Handle authorization code flow, token validation, session management.
- All requests require authentication; reject unauthenticated with `401`.
- Populate `owner_id` on every paste from the authenticated user.
- Add "my pastes" API endpoint (`GET /api/v1/pastes?mine=true`).
- Security: CSRF tokens, secure cookies, CORS configuration.
- Write tests covering auth flow and authorization checks.

### Phase 4: Frontend (React + Pragma)

- Initialize React + TypeScript project in `web/`.
- Install Pragma[^2] components (`@canonical/react-components`).
- Implement pages: New Paste form, Paste Viewer (with syntax highlighting), 404 state.
- Integrate client-side syntax highlighting (e.g. `react-syntax-highlighter`).
- Wire API calls to the Go backend.
- Implement input/output hardening (§8): sanitize before render, validate API
  responses, no `dangerouslySetInnerHTML`.
- Implement authenticated "my pastes" view.
- Write unit tests (Vitest) and e2e tests (Playwright) for all user workflows.

### Phase 5: 12-Factor Finalization & Charm

- Verify all config read from environment variables; generate `.env.example`.
- Verify all logs stream structured JSON to stdout/stderr.
- Add security headers middleware (CSP, X-Frame-Options, X-Content-Type-Options).
- Write `bingo-rockcraft.yaml` building the Go binary + frontend static assets.
- Write `charmcraft.yaml` using `go-framework` extension.
- Write charm Python source (`src/charm.py`) extending `paas_charm.go.Charm`.
- Write `artifacts.yaml`, `spread.yaml`, `concierge.yaml`.
- Write charm unit tests (Scenario framework) and integration tests.
- Configure all GitHub Actions workflows.

---

## References

[^1]: dpaste — <https://github.com/DarrenOfficial/dpaste>
[^2]: Pragma — <https://github.com/canonical/pragma>
[^3]: github-runner-operators — <https://github.com/canonical/github-runner-operators>
[^4]: Go community project layout — <https://github.com/golang-standards/project-layout>
[^5]: pgx — <https://github.com/jackc/pgx>
[^6]: golang-migrate — <https://github.com/golang-migrate/migrate>
[^7]: charm-ci — <https://github.com/canonical/charm-ci>
[^8]: Superpowers — <https://github.com/obra/Superpowers>
[^9]: Google Go Style Guide — <https://google.github.io/styleguide/go/>
[^10]: Pebble — <https://github.com/canonical/pebble>