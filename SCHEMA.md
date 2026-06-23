# Pastebin (Go + PostgreSQL) — Schema & JSON API Plan

A Go-based pastebin replacing the legacy pastebin.canonical.com, modeled on dpaste.
Strict JSON API (no server-side HTML). PostgreSQL is the source of truth; content
stored inline in `TEXT`.

## Decisions

- **Anonymous MVP** — no user accounts. Deletion via a one-time secret token shown
  once at creation. Future user ownership reserved via a nullable `owner_id`.
- **Immutable pastes** — create + delete only. No edit/PUT.
- **No parent/child chains.**
- **Content in Postgres `TEXT`** — no S3 in MVP.
- **Syntax highlighting via Chroma** (Go port of Pygments). `/languages` is generated
  from the Chroma registry at runtime (single source of truth); submitted `language`
  is validated against it. NOTE: the legacy Canonical dropdown was a full Pygments
  dump (not curated) — keep it only as a migration mapping reference.
- **Max content size 5 MiB** (`5242880` bytes). Comfortably fits Postgres TEXT/TOAST.
- **Retention** — every paste expires (no keep-forever). Allowed durations:
  `1d`, `1w`, `1mo`, `3mo` (default), `1y` (max).
- **Rate limiting** — per-IP token bucket at the API gateway (out of app scope).
- Timestamps are `TIMESTAMPTZ` in UTC.

## Database Schema

```sql
CREATE TABLE pastes (
    id                BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug              TEXT        NOT NULL UNIQUE,
    content           TEXT        NOT NULL,
    language          VARCHAR(64) NOT NULL DEFAULT 'plaintext',
    title             VARCHAR(255),
    size_bytes        INTEGER     NOT NULL,
    burn_after_read   BOOLEAN     NOT NULL DEFAULT false,
    expires_at        TIMESTAMPTZ NOT NULL,            -- always set; no keep-forever
    view_count        INTEGER     NOT NULL DEFAULT 0,
    view_limit        INTEGER,                         -- set iff burn_after_read
    delete_token_hash BYTEA,                           -- SHA-256 of delete token
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- owner_id       BIGINT REFERENCES users(id),     -- reserved for future auth

    CONSTRAINT view_limit_consistent CHECK (
        (burn_after_read     AND view_limit IS NOT NULL AND view_limit >= 1) OR
        (NOT burn_after_read AND view_limit IS NULL)
    ),
    CONSTRAINT view_count_nonneg     CHECK (view_count >= 0),
    CONSTRAINT size_within_limit     CHECK (size_bytes BETWEEN 1 AND 5242880), -- 5 MiB
    CONSTRAINT slug_length           CHECK (char_length(slug) BETWEEN 4 AND 32),
    CONSTRAINT expiry_after_creation CHECK (expires_at > created_at)
);

CREATE INDEX pastes_expires_at_idx ON pastes (expires_at);  -- background sweep
```

### Field reference

| Field | Type | Notes |
|---|---|---|
| `id` | `bigint` identity | Internal PK; never exposed in API |
| `slug` | `text` unique | Public base62 ID; collision-retry lengthens it |
| `content` | `text` | Paste body, ≤ 5 MiB |
| `language` | `varchar(64)` | Chroma lexer key; default `plaintext` |
| `title` | `varchar(255)` null | Optional |
| `size_bytes` | `int` | `octet_length(content)` |
| `burn_after_read` | `bool` | One-time semantics |
| `expires_at` | `timestamptz` | Always set; ≤ `created_at` + 1 year |
| `view_count` | `int` | Incremented per retrieval |
| `view_limit` | `int` null | Set iff `burn_after_read`; default 1 |
| `delete_token_hash` | `bytea` null | SHA-256 of token shown once at create |
| `created_at` | `timestamptz` | UTC |

## Business-Logic Constraints (enforced in Go)

1. **Slug generation** — base62, default length 4; on `UNIQUE` violation retry with
   length + 1 (dpaste pattern).
2. **Burn-after-read** — transactional:
   ```sql
   BEGIN;
   SELECT ... FROM pastes WHERE slug = $1 FOR UPDATE;
   UPDATE pastes SET view_count = view_count + 1 WHERE id = $1;
   -- return content, then if view_count >= view_limit: DELETE WHERE id = $1;
   COMMIT;
   ```
   Atomic read-then-burn; concurrent readers cannot both win.
3. **Expiry** — lazy (`GET` past `expires_at` → `404` + delete) plus background sweep:
   `DELETE FROM pastes WHERE expires_at < now();`
4. **Retention validation** — `expires_in` must be one of the allowed durations;
   default `3mo`, hard max `1y`.
5. **Delete token** — 32 random bytes (base64url), stored only as SHA-256, compared in
   constant time; returned once at create.
6. **Boundary validation** — reject empty content, content > 5 MiB (`413`), unknown
   `language`, invalid `expires_in`.

## JSON API (`/api/v1`)

All requests/responses are `application/json` (except `/raw`). Timestamps are RFC 3339 UTC.

### `POST /api/v1/pastes` — create

Request:
```json
{
  "content": "print('hello')",
  "language": "python",
  "title": "demo snippet",
  "expires_in": "3mo",
  "burn_after_read": false
}
```
- `content` (required, 1 byte – 5 MiB)
- `language` (optional, default `"plaintext"`; validated against Chroma)
- `title` (optional, ≤ 255 chars)
- `expires_in` (optional enum: `"1d"`, `"1w"`, `"1mo"`, `"3mo"` (default), `"1y"`)
- `burn_after_read` (optional bool, default `false`)
- `view_limit` (optional int ≥ 1; only honored when `burn_after_read=true`, default 1)

Response `201 Created`:
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
  "created_at": "2026-06-23T12:00:00Z",
  "delete_token": "Yk9f...base64url...Qw"
}
```
> `delete_token` is returned **only** here.

### `GET /api/v1/pastes/{slug}` — retrieve

Response `200 OK`:
```json
{
  "slug": "aB3xY",
  "content": "print('hello')",
  "language": "python",
  "title": "demo snippet",
  "size_bytes": 14,
  "burn_after_read": false,
  "expires_at": "2026-09-21T12:00:00Z",
  "view_count": 3,
  "remaining_views": null,
  "created_at": "2026-06-23T12:00:00Z"
}
```
- For burn pastes, `remaining_views` = `view_limit - view_count`; this request consumes
  a view and may delete the paste afterward.
- Expired/missing → `404`.

### `GET /api/v1/pastes/{slug}/raw`
`text/plain; charset=utf-8` body only (curl-friendly). Same burn/expiry semantics.

### `DELETE /api/v1/pastes/{slug}`
Requires `Authorization: Bearer <delete_token>`. Returns `204 No Content`; bad/missing
token → `403`.

### `GET /api/v1/languages`
Generated from the Chroma registry:
```json
{ "languages": [ { "key": "python", "name": "Python" }, { "key": "go", "name": "Go" } ] }
```

### `GET /api/v1/healthz`
Liveness + DB ping.

### Error envelope (all 4xx/5xx)
```json
{ "error": { "code": "content_too_large", "message": "Paste exceeds the 5 MiB limit." } }
```

| Status | `code` examples |
|---|---|
| 400 | `invalid_request`, `missing_content`, `invalid_expires_in`, `unknown_language` |
| 403 | `invalid_delete_token` |
| 404 | `paste_not_found` (also expired/burned) |
| 413 | `content_too_large` |
| 429 | `rate_limited` (emitted by gateway) |
| 500 | `internal_error` |

## Verification

1. Migration applies cleanly; CHECK constraints reject bad rows (e.g.
   `burn_after_read=true` with null `view_limit`; `expires_at <= created_at`;
   `size_bytes > 5242880`).
2. Concurrent reads of a burn paste yield exactly one body, then `404`.
3. Sweep deletes only past-due pastes; `expires_in` beyond `1y` rejected at the API.
4. `DELETE` with wrong token → `403`; correct token → `204`, then `404` on refetch.

## Out of Scope (MVP)

- User accounts / authentication (schema reserves `owner_id`).
- Paste editing, parent/child reply chains.
- S3 / object-storage offload (not needed at 5 MiB).
- In-app rate limiting (handled at the gateway).
