# WORKFLOW.md: bingo User Workflow

Reference for the end-user UI flow, reconciled against [SCHEMA.md](SCHEMA.md) and
[PLAN.md](PLAN.md). Modeled on the legacy Canonical Pastebin and `dpaste`, adapted to
bingo's anonymous-MVP, JSON-API architecture.

## MVP Workflow

```
- Auth (OIDC) is OUT of MVP. No user accounts (SCHEMA.md: "Anonymous MVP").
  - "Ownership" in MVP = a one-time delete token, not a login.

- Default page (default URL) is the "new paste" page.
    - ex. https://paste.canonical.com/
    - Fields: Title (optional), Syntax, Expiration, Content + Submit button.
      (Note: legacy "Poster" field is dropped; bingo uses an optional Title instead.)
    - Optional: Burn-after-read (+ view limit).

- Upon creating a new paste, redirect to that paste's unique URL & display its content.
    - Canonical new URL: https://paste.canonical.com/{slug}   (no /p/)
    - Legacy URL still resolves: https://pastebin.canonical.com/p/{id}/
      and https://paste.ubuntu.com/p/{id}/ (backwards-compat routing, PLAN.md §5).
    - View page shows: creation date, expiry date, syntax type, paste content,
      view raw, toggle wrap, copy to clipboard.
      - "view raw" -> GET /api/v1/pastes/{slug}/raw
      - "toggle wrap" and "copy" are client-side only.
    - The one-time delete token is shown ONCE here, prominently. It is the only way
      to delete the paste (DELETE with `Authorization: Bearer <token>`). It is never
      shown again. This is the MVP stand-in for owner disable/enable access.
    - For burn pastes, also show remaining_views.

- Navigating to a paste's unique URL directly shows the same view page as above.
    - Expired / missing / burned-out pastes return 404 -> show a
      "not found / expired" state.

- "new paste" link is also visible when viewing an existing paste (at the bottom).
```

## Out of Scope (MVP)

- OIDC / user accounts / authentication (schema reserves `owner_id` for future use).
- Owner-driven disable/enable access (replaced by the one-time delete token in MVP).
- Paste editing (pastes are immutable: create + delete only).

## Field & Routing Notes

- **Create form fields:** Title (optional, ≤255 chars), Syntax (validated against the
  Chroma registry via `GET /api/v1/languages`), Expiration, Content (1 byte – 5 MiB).
  Optional: `burn_after_read` (+ `view_limit`).
- **Expiration options (fixed enum):** `1d`, `1w`, `1mo`, `3mo` (default), `1y` (max).
- **Canonical URL:** `https://paste.canonical.com/{slug}`; redirect target after create
  is `/{slug}`.
- **Legacy URLs (must still resolve):** `/p/{id}/` on both `pastebin.canonical.com`
  and `paste.ubuntu.com`.

## Relevant API Endpoints

- `POST /api/v1/pastes` — create; returns `slug`, `url`, `raw_url`, metadata, and the
  one-time `delete_token`.
- `GET /api/v1/pastes/{slug}` — retrieve content + metadata (`remaining_views` for burn
  pastes); expired/missing → `404`.
- `GET /api/v1/pastes/{slug}/raw` — raw `text/plain` body (curl-friendly).
- `DELETE /api/v1/pastes/{slug}` — requires `Authorization: Bearer <delete_token>`.
- `GET /api/v1/languages` — syntax options for the form.
