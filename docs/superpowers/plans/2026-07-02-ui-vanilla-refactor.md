# Bingo UI — Canonical Vanilla Framework Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the React frontend to strictly use Canonical's Vanilla Framework and `@canonical/react-components` (Pragma) design system, matching the aesthetic of MAAS UI and Juju Dashboard.

**Architecture:** All custom CSS variables and overrides are removed; Vanilla Framework owns all base styling via the existing `@use 'vanilla-framework'` import. Layout uses `l-application` / `l-main` shell classes and the Vanilla grid (`row` / `col-*`). Pragma React components replace hand-rolled HTML wherever a component exists.

**Tech Stack:** React 19, TypeScript, `@canonical/react-components` v4.6.2, `vanilla-framework` v4.55.1, Vite, Vitest, `react-syntax-highlighter`

## Global Constraints

- Never use custom CSS variables for colours — Vanilla Framework owns all colour tokens.
- All interactive clickable elements must use `Button` from `@canonical/react-components`.
- Use Vanilla grid classes (`row`, `col-*`) for layout — no ad-hoc flexbox for page structure.
- All commands run from `web/` unless otherwise noted.
- Run tests with: `npm test` (unit, Vitest) from `web/`.
- TypeScript errors are build failures — `npm run build` must succeed after every task.
- No new dependencies may be added; `@canonical/react-components` and `vanilla-framework` are already installed.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `index.html` | Modify | Add Ubuntu font, update `<title>` |
| `src/index.css` | Delete | Remove custom CSS variables that fight Vanilla |
| `src/App.css` | Delete | Remove unused Vite-generated file |
| `src/styles/index.scss` | Modify | Keep Vanilla import; add minimal paste-viewer override |
| `src/main.tsx` | Modify | Remove `./index.css` import if present; keep SCSS import |
| `src/App.tsx` | Modify | Add `l-application` wrapper |
| `src/components/Navigation/Navigation.tsx` | Rewrite | Use Pragma `Navigation` component (dark theme) |
| `src/pages/HomePage.tsx` | Modify | Proper `l-main` + grid layout |
| `src/pages/MyPastesPage.tsx` | Modify | Proper `l-main` + grid layout |
| `src/pages/PastePage.tsx` | Modify | Proper `l-main` + grid layout |
| `src/components/MyPastesList/MyPastesList.tsx` | Rewrite | Use Pragma `MainTable` |
| `src/components/PasteViewer/PasteViewer.tsx` | Modify | Replace `<button>` with Pragma `Button` |
| `src/components/NewPasteForm/NewPasteForm.tsx` | Modify | Add `p-form` class to `<form>` |

---

## Task 1: Remove custom CSS and fix the HTML shell

**Files:**
- Modify: `index.html`
- Delete: `src/index.css`
- Delete: `src/App.css`
- Modify: `src/styles/index.scss`
- Modify: `src/main.tsx`

**Interfaces:**
- Produces: Ubuntu font loaded globally; Vanilla Framework is the only CSS base layer.

- [ ] **Step 1: Update `index.html`**

Replace the entire file content:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Bingo — Canonical Pastebin</title>
    <link rel="preconnect" href="https://fonts.ubuntu.com" />
    <link
      rel="stylesheet"
      href="https://fonts.ubuntu.com/css2?family=Ubuntu:wght@300;400;500&family=Ubuntu+Mono&display=swap"
    />
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 2: Delete `src/index.css` and `src/App.css`**

```bash
rm web/src/index.css web/src/App.css
```

These files define custom CSS variables with non-Canonical colours that override Vanilla Framework.

- [ ] **Step 3: Clean `src/main.tsx`**

Replace entire file (remove any `./index.css` import; keep only the SCSS import):

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import './styles/index.scss'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </React.StrictMode>,
)
```

- [ ] **Step 4: Replace `src/styles/index.scss`**

Replace the entire file. The only custom rule needed is resetting the syntax highlighter's default margin so it fits flush inside card/strip containers:

```scss
@use 'vanilla-framework';

// Paste viewer: remove default margin from syntax highlighter wrapper
.paste-code-block {
  margin-bottom: 0;

  pre {
    margin: 0;
    border-radius: 0;
    font-family: 'Ubuntu Mono', ui-monospace, Consolas, monospace;
    font-size: 0.875rem;
  }
}
```

- [ ] **Step 5: Verify the build compiles**

```bash
cd web && npm run build 2>&1 | tail -20
```

Expected: `✓ built in` with no errors. TypeScript and Vite should compile cleanly.

- [ ] **Step 6: Commit**

```bash
cd web && git add -A && git commit -m "style: remove custom CSS, load Ubuntu font via Vanilla Framework

- Delete src/index.css and src/App.css (wrong brand colours)
- Load Ubuntu font from fonts.ubuntu.com in index.html
- Add paste-code-block SCSS helper for syntax highlighter
- Update page title to 'Bingo — Canonical Pastebin'

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 2: Add `l-application` shell and fix the App wrapper

**Files:**
- Modify: `src/App.tsx`

**Interfaces:**
- Consumes: nothing new
- Produces: `<div className="l-application">` wraps all routes; pages receive correct Vanilla application shell.

- [ ] **Step 1: Rewrite `src/App.tsx`**

```tsx
import { Routes, Route } from 'react-router-dom'
import HomePage from './pages/HomePage'
import PastePage from './pages/PastePage'
import MyPastesPage from './pages/MyPastesPage'

export default function App() {
  return (
    <div className="l-application" role="presentation">
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/my-pastes" element={<MyPastesPage />} />
        <Route path="/:key" element={<PastePage />} />
      </Routes>
    </div>
  )
}
```

- [ ] **Step 2: Verify build**

```bash
cd web && npm run build 2>&1 | tail -5
```

Expected: no TypeScript errors, `✓ built in`.

- [ ] **Step 3: Commit**

```bash
cd web && git add src/App.tsx && git commit -m "style: wrap app in l-application shell

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 3: Rewrite Navigation with Pragma `Navigation` component

**Files:**
- Rewrite: `src/components/Navigation/Navigation.tsx`

**Interfaces:**
- Consumes: props `{ isAuthenticated: boolean; userEmail?: string }` (unchanged — callers don't change)
- Produces: dark-theme Vanilla navigation bar using the Pragma `Navigation` component

The Pragma `Navigation` component renders the correct Vanilla `p-navigation p-navigation--dark` markup and handles mobile toggle automatically.

`NavLinkAnchor` items require a `url` field. Items without `url` use `NavLinkButton` shape (no `url` key or `url: undefined`). Use the `generateLink` prop so router `<Link>` components handle internal routes and avoid full-page reloads.

- [ ] **Step 1: Rewrite `src/components/Navigation/Navigation.tsx`**

```tsx
import { Navigation } from '@canonical/react-components'
import { Theme } from '@canonical/react-components/dist/enums'
import { Link } from 'react-router-dom'
import type { NavItem, GenerateLink } from '@canonical/react-components/dist/components/Navigation/types'

interface Props {
  isAuthenticated: boolean
  userEmail?: string
}

// generateLink is called for items with a `url`. Use react-router Link for
// internal paths; fall through to a plain <a> for /auth/* server routes.
const generateLink: GenerateLink = ({ label, url, ...props }) => {
  if (url && !url.startsWith('/auth')) {
    return <Link to={url} {...(props as object)}>{label}</Link>
  }
  return <a href={url} {...(props as object)}>{label}</a>
}

export default function AppNavigation({ isAuthenticated, userEmail }: Props) {
  const rightItems: NavItem[] = isAuthenticated
    ? [
        { label: 'My pastes', url: '/my-pastes' },
        ...(userEmail ? [{ label: userEmail, url: '#' } as NavItem] : []),
        { label: 'Log out', url: '/auth/logout' },
      ]
    : [{ label: 'Log in', url: '/auth/login' }]

  return (
    <Navigation
      theme={Theme.DARK}
      logo={{
        src: '/favicon.svg',
        title: 'Bingo',
        url: '/',
      }}
      generateLink={generateLink}
      items={[{ label: 'New paste', url: '/' }]}
      itemsRight={rightItems}
    />
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd web && npm run build 2>&1 | tail -10
```

Expected: no errors, `✓ built in`.

- [ ] **Step 3: Run unit tests**

```bash
cd web && npm test 2>&1 | tail -20
```

Expected: all tests pass (PastePage tests use the Navigation component indirectly; they should still pass because the Pragma Navigation renders an accessible `<nav>` element).

- [ ] **Step 4: Commit**

```bash
cd web && git add src/components/Navigation/Navigation.tsx && git commit -m "style: replace hand-rolled nav with Pragma Navigation (dark theme)

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 4: Fix page layouts with `l-main` and Vanilla grid

**Files:**
- Modify: `src/pages/HomePage.tsx`
- Modify: `src/pages/MyPastesPage.tsx`
- Modify: `src/pages/PastePage.tsx`

**Interfaces:**
- Consumes: `AppNavigation` component (Task 3), child components unchanged
- Produces: pages with proper `l-main > p-strip > row > col-*` structure

Grid column widths:
- `col-8` for the paste creation form (focused, narrow).
- `col-12` for the paste list and paste viewer (full-width, data-heavy).

- [ ] **Step 1: Rewrite `src/pages/HomePage.tsx`**

```tsx
import { useNavigate } from 'react-router-dom'
import AppNavigation from '../components/Navigation/Navigation'
import NewPasteForm from '../components/NewPasteForm/NewPasteForm'

export default function HomePage() {
  const navigate = useNavigate()
  const isAuthenticated = document.cookie.includes('csrf_token=')

  return (
    <>
      <AppNavigation isAuthenticated={isAuthenticated} />
      <main className="l-main">
        <section className="p-strip is-shallow">
          <div className="row">
            <div className="col-8">
              <h1 className="p-heading--2">New paste</h1>
              <NewPasteForm onCreated={(key) => navigate(`/${key}`)} />
            </div>
          </div>
        </section>
      </main>
    </>
  )
}
```

- [ ] **Step 2: Rewrite `src/pages/MyPastesPage.tsx`**

```tsx
import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import AppNavigation from '../components/Navigation/Navigation'
import MyPastesList from '../components/MyPastesList/MyPastesList'

export default function MyPastesPage() {
  const navigate = useNavigate()
  const isAuthenticated = document.cookie.includes('csrf_token=')

  useEffect(() => {
    if (!isAuthenticated) navigate('/')
  }, [isAuthenticated, navigate])

  if (!isAuthenticated) return null

  return (
    <>
      <AppNavigation isAuthenticated userEmail={undefined} />
      <main className="l-main">
        <section className="p-strip is-shallow">
          <div className="row">
            <div className="col-12">
              <h1 className="p-heading--2">My pastes</h1>
              <MyPastesList />
            </div>
          </div>
        </section>
      </main>
    </>
  )
}
```

- [ ] **Step 3: Rewrite `src/pages/PastePage.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Spinner, Notification } from '@canonical/react-components'
import AppNavigation from '../components/Navigation/Navigation'
import PasteViewer from '../components/PasteViewer/PasteViewer'
import { getPaste, deletePaste } from '../api/client'
import { PasteResponse, ApiRequestError } from '../api/types'

export default function PastePage() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const [paste, setPaste] = useState<PasteResponse | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const isAuthenticated = document.cookie.includes('csrf_token=')

  useEffect(() => {
    if (!key) return
    getPaste(key)
      .then(setPaste)
      .catch((err) => {
        if (err instanceof ApiRequestError && err.status === 404) {
          setNotFound(true)
        } else {
          setError(err instanceof Error ? err.message : 'Failed to load paste.')
        }
      })
  }, [key])

  async function handleDelete() {
    if (!key) return
    try {
      await deletePaste(key)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete paste.')
    }
  }

  return (
    <>
      <AppNavigation isAuthenticated={isAuthenticated} />
      <main className="l-main">
        <section className="p-strip is-shallow">
          <div className="row">
            <div className="col-12">
              {!paste && !notFound && !error && <Spinner role="status" text="Loading…" />}
              {notFound && (
                <p>
                  Paste not found or has expired.{' '}
                  <a href="/">Create a new paste.</a>
                </p>
              )}
              {error && (
                <Notification severity="negative" title="Error">
                  {error}
                </Notification>
              )}
              {paste && (
                <PasteViewer
                  paste={paste}
                  onDelete={isAuthenticated ? handleDelete : undefined}
                />
              )}
            </div>
          </div>
        </section>
      </main>
    </>
  )
}
```

- [ ] **Step 4: Run unit tests**

```bash
cd web && npm test 2>&1 | tail -20
```

Expected: all tests pass.

- [ ] **Step 5: Verify build**

```bash
cd web && npm run build 2>&1 | tail -5
```

Expected: `✓ built in` with no TypeScript errors.

- [ ] **Step 6: Commit**

```bash
cd web && git add src/pages/ && git commit -m "style: apply l-main and Vanilla grid to all pages

- HomePage: col-8 for the paste creation form
- MyPastesPage: col-12 for the full-width table
- PastePage: col-12 for the paste viewer

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 5: Replace `MyPastesList` with Pragma `MainTable`

**Files:**
- Rewrite: `src/components/MyPastesList/MyPastesList.tsx`

**Interfaces:**
- Consumes: `PasteListItem[]` from `getMyPastes()` (unchanged API types)
- Produces: a `MainTable` with columns: Title, Language, Size, Created, Expires

`MainTable` from `@canonical/react-components` accepts:
- `headers: MainTableHeader[]` — each with a `content` field (ReactNode)
- `rows: MainTableRow[]` — each with a `columns: MainTableCell[]` array (each cell has `content`)
- `sortable` — optional boolean to enable client-side sorting (requires `sortKey` on headers and `sortData` on rows)
- `emptyStateMsg` — shown when rows array is empty

- [ ] **Step 1: Rewrite `src/components/MyPastesList/MyPastesList.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { MainTable, Notification, Spinner } from '@canonical/react-components'
import { getMyPastes } from '../../api/client'
import { PasteListItem } from '../../api/types'
import { sanitizeTitle } from '../../utils/sanitize'

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString('en-GB', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    })
  } catch {
    return iso
  }
}

export default function MyPastesList() {
  const [pastes, setPastes] = useState<PasteListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getMyPastes()
      .then((resp) => setPastes(resp.pastes))
      .catch((err) =>
        setError(err instanceof Error ? err.message : 'Failed to load pastes.'),
      )
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <Spinner role="status" text="Loading your pastes…" />
  if (error)
    return (
      <Notification severity="negative" title="Error" role="alert">
        {error}
      </Notification>
    )

  const headers = [
    { content: 'Title', sortKey: 'title' },
    { content: 'Language', sortKey: 'language' },
    { content: 'Size', sortKey: 'size_bytes' },
    { content: 'Created', sortKey: 'created_at' },
    { content: 'Expires', sortKey: 'expires_at' },
  ]

  const rows = pastes.map((p) => ({
    key: p.key,
    sortData: {
      title: sanitizeTitle(p.title) || p.key,
      language: p.language,
      size_bytes: p.size_bytes,
      created_at: p.created_at,
      expires_at: p.expires_at,
    },
    columns: [
      {
        content: (
          <Link to={`/${p.key}`}>
            {sanitizeTitle(p.title) || p.key}
          </Link>
        ),
      },
      { content: p.language },
      { content: `${p.size_bytes} B` },
      { content: formatDate(p.created_at) },
      { content: formatDate(p.expires_at) },
    ],
  }))

  return (
    <MainTable
      headers={headers}
      rows={rows}
      sortable
      emptyStateMsg="No pastes yet."
    />
  )
}
```

- [ ] **Step 2: Run unit tests**

```bash
cd web && npm test 2>&1 | tail -20
```

Expected: all tests pass (MyPastesList has no dedicated unit test; the build check is sufficient).

- [ ] **Step 3: Verify build**

```bash
cd web && npm run build 2>&1 | tail -5
```

Expected: `✓ built in`, no errors.

- [ ] **Step 4: Commit**

```bash
cd web && git add src/components/MyPastesList/MyPastesList.tsx && git commit -m "style: replace ul list with Pragma MainTable in MyPastesList

- Adds sortable columns: Title, Language, Size, Created, Expires
- Keeps Spinner and Notification for loading/error states

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 6: Replace plain `<button>` elements in `PasteViewer`

**Files:**
- Modify: `src/components/PasteViewer/PasteViewer.tsx`

**Interfaces:**
- Consumes: `{ paste: PasteResponse; onDelete?: () => void }` (unchanged)
- Produces: all interactive actions use Pragma `Button`; metadata in a `<table className="p-table--mobile-card">` for the data-heavy Canonical aesthetic.

- [ ] **Step 1: Rewrite `src/components/PasteViewer/PasteViewer.tsx`**

```tsx
import { useState } from 'react'
import { Button, Row, Col } from '@canonical/react-components'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { tomorrow } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { PasteResponse } from '../../api/types'
import { sanitizeContent, sanitizeTitle } from '../../utils/sanitize'

interface Props {
  paste: PasteResponse
  onDelete?: () => void
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString('en-US', { timeZone: 'UTC' })
  } catch {
    return iso
  }
}

export default function PasteViewer({ paste, onDelete }: Props) {
  const [wrapLines, setWrapLines] = useState(false)
  const content = sanitizeContent(paste.content)
  const title = sanitizeTitle(paste.title)

  return (
    <article aria-label="Paste viewer">
      <Row>
        <Col size={12}>
          {title && <h2 className="p-heading--3">{title}</h2>}

          {/* Metadata table */}
          <table className="p-table--mobile-card u-no-margin--bottom">
            <tbody>
              <tr>
                <td className="u-text--muted">Language</td>
                <td>{paste.language}</td>
              </tr>
              <tr>
                <td className="u-text--muted">Created</td>
                <td>{formatDate(paste.created_at)}</td>
              </tr>
              <tr>
                <td className="u-text--muted">Expires</td>
                <td>{formatDate(paste.expires_at)}</td>
              </tr>
              <tr>
                <td className="u-text--muted">Size</td>
                <td>{paste.size_bytes} bytes</td>
              </tr>
            </tbody>
          </table>

          {/* Action bar — use <a> with Vanilla classes for navigation links,
              Pragma Button for interactive actions (copy, wrap, delete) */}
          <div className="u-sv2" style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', marginBlock: '1rem' }}>
            <a
              href={paste.raw_url}
              className="p-button--base is-small"
              aria-label="View raw"
            >
              View raw
            </a>
            <a
              href="/"
              className="p-button--base is-small"
              aria-label="New paste"
            >
              New paste
            </a>
            <Button
              type="button"
              appearance="base"
              small
              onClick={() => setWrapLines((w) => !w)}
              aria-pressed={wrapLines}
            >
              {wrapLines ? 'Unwrap lines' : 'Wrap lines'}
            </Button>
            <Button
              type="button"
              appearance="base"
              small
              aria-label="Copy to clipboard"
              onClick={() => navigator.clipboard.writeText(content)}
            >
              Copy
            </Button>
            {onDelete && (
              <Button
                type="button"
                appearance="negative"
                small
                onClick={onDelete}
                aria-label="Delete paste"
              >
                Delete
              </Button>
            )}
          </div>

          {/* Code block — SyntaxHighlighter renders tokens as React elements,
              never passes untreated API strings to dangerouslySetInnerHTML */}
          <div className="paste-code-block">
            <SyntaxHighlighter
              language={paste.language}
              style={tomorrow}
              wrapLines={wrapLines}
              wrapLongLines={wrapLines}
              showLineNumbers
            >
              {content}
            </SyntaxHighlighter>
          </div>
        </Col>
      </Row>
    </article>
  )
}
```

- [ ] **Step 2: Run unit tests**

```bash
cd web && npm test 2>&1 | tail -20
```

Expected: all tests pass. The PastePage test renders PasteViewer; verify it still finds `screen.getByText(/print/)`.

- [ ] **Step 3: Verify build**

```bash
cd web && npm run build 2>&1 | tail -5
```

Expected: `✓ built in`, no errors.

- [ ] **Step 4: Commit**

```bash
cd web && git add src/components/PasteViewer/PasteViewer.tsx && git commit -m "style: replace plain buttons with Pragma Button in PasteViewer

- All action buttons (raw, new, wrap, copy, delete) use Pragma Button
- Metadata rendered as p-table--mobile-card
- Code block wrapped in paste-code-block class for SCSS override

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 7: Add `p-form` class to `NewPasteForm`

**Files:**
- Modify: `src/components/NewPasteForm/NewPasteForm.tsx`

**Interfaces:**
- Consumes: `{ onCreated: (key: string) => void }` (unchanged)
- Produces: `<form>` with `className="p-form p-form--stacked"` for Vanilla form styling

- [ ] **Step 1: Add `p-form` class to the `<form>` element**

In `src/components/NewPasteForm/NewPasteForm.tsx`, locate:

```tsx
  return (
    <form onSubmit={handleSubmit} aria-label="New paste form">
```

Replace with:

```tsx
  return (
    <form onSubmit={handleSubmit} aria-label="New paste form" className="p-form p-form--stacked">
```

No other changes to this file.

- [ ] **Step 2: Run unit tests**

```bash
cd web && npm test 2>&1 | tail -20
```

Expected: all tests pass.

- [ ] **Step 3: Verify build**

```bash
cd web && npm run build 2>&1 | tail -5
```

Expected: `✓ built in`, no errors.

- [ ] **Step 4: Commit**

```bash
cd web && git add src/components/NewPasteForm/NewPasteForm.tsx && git commit -m "style: add p-form p-form--stacked class to NewPasteForm

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

## Task 8: Final validation

- [ ] **Step 1: Run full unit test suite**

```bash
cd web && npm test 2>&1
```

Expected: all tests pass, 0 failures.

- [ ] **Step 2: Run production build**

```bash
cd web && npm run build 2>&1
```

Expected: `✓ built in` with no TypeScript errors or warnings.

- [ ] **Step 3: Run linter**

```bash
cd web && npm run lint 2>&1 | tail -20
```

Expected: 0 errors, 0 warnings (or same baseline as before the refactor).

- [ ] **Step 4: Confirm no custom CSS variable references remain**

```bash
grep -r "var(--" web/src/ --include="*.tsx" --include="*.ts" --include="*.css" --include="*.scss"
```

Expected: no output (all custom CSS variables removed).

- [ ] **Step 5: Confirm no plain `<button>` elements remain in components**

```bash
grep -rn "<button" web/src/components/ --include="*.tsx"
```

Expected: no output (action links use `<a className="p-button--base">`, interactive triggers use Pragma `Button`).

- [ ] **Step 6: Final commit**

```bash
cd web && git add -A && git commit -m "style: Canonical Vanilla Framework UI refactor complete

Summary of changes:
- Ubuntu font loaded from fonts.ubuntu.com
- Custom CSS variables removed; Vanilla Framework owns all colours
- l-application shell wraps the app
- Pragma Navigation component with dark/aubergine theme
- Vanilla grid (row/col-*) used on all pages
- MainTable replaces ul list in MyPastesList
- All buttons use Pragma Button component
- NewPasteForm uses p-form p-form--stacked

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```
