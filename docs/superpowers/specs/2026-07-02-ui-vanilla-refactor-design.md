# Design Spec: Bingo UI — Canonical Vanilla Framework Refactor

**Date:** 2026-07-02  
**Status:** Approved  
**Scope:** React frontend (`web/`) — full visual refactor to Canonical design system

---

## 1. Problem Statement

The current frontend UI is visually inconsistent with Canonical's design system. It uses
custom CSS variables with non-Canonical colors (purple accent `#aa3bff`), does not load
the Ubuntu font, hand-rolls navigation instead of using Pragma components, and uses plain
HTML elements where Pragma/Vanilla Framework components should be used.

---

## 2. Goals

- Load the Ubuntu font family correctly.
- Remove all custom CSS that conflicts with Vanilla Framework.
- Use `l-application` / `l-main` structural layout classes.
- Use the `Navigation` Pragma component with a dark/aubergine theme.
- Use the `MainTable` Pragma component for the "My Pastes" list.
- Use `Button` from Pragma for every interactive action.
- Use Vanilla grid (`row`, `col-*`) for spacing the paste form and viewer.
- Match the clean, data-heavy aesthetic of MAAS UI and Juju Dashboard.

---

## 3. Non-Goals

- Adding new features or changing the API layer.
- Adding a sidebar navigation (out of scope; overkill for a pastebin).
- Dark mode toggle (Vanilla Framework handles system dark mode automatically).
- Changing routing logic or authentication flow.

---

## 4. Changes by File

### 4.1 `web/index.html`

- Set `<title>` to `Bingo — Canonical Pastebin`.
- Add Ubuntu font via `<link>` preconnect + stylesheet from `fonts.ubuntu.com`:
  ```html
  <link rel="preconnect" href="https://fonts.ubuntu.com" />
  <link rel="stylesheet" href="https://fonts.ubuntu.com/css2?family=Ubuntu:wght@300;400;500&family=Ubuntu+Mono&display=swap" />
  ```

### 4.2 `web/src/index.css`

**DELETE this file.** It defines custom CSS variables with wrong brand colors and overrides
Vanilla Framework. The `styles/index.scss` → `@use 'vanilla-framework'` already provides
all necessary base styles.

Any import of `index.css` in `main.tsx` must be removed.

### 4.3 `web/src/styles/index.scss`

Keep `@use 'vanilla-framework'` as the only import. Add **only** the minimal
application-specific overrides needed (e.g., code block styling for the paste viewer),
using Vanilla's SCSS variables/settings where possible.

Avoid custom CSS variables. Use Vanilla's colour tokens (`$color-brand`, etc.).

### 4.4 `web/src/App.tsx`

Wrap the `<Routes>` in a `<div className="l-application">` so every page inherits the
correct Vanilla application shell.

```tsx
export default function App() {
  return (
    <div className="l-application">
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/my-pastes" element={<MyPastesPage />} />
        <Route path="/:key" element={<PastePage />} />
      </Routes>
    </div>
  )
}
```

### 4.5 `web/src/components/Navigation/Navigation.tsx`

Replace the hand-rolled `<nav>` with the `Navigation` Pragma component:

```tsx
import { Navigation } from '@canonical/react-components'
import { Theme } from '@canonical/react-components/dist/enums'
import { Link } from 'react-router-dom'

export default function AppNav({ isAuthenticated, userEmail }) {
  return (
    <Navigation
      theme={Theme.DARK}
      logo={{ src: '/favicon.svg', title: 'Bingo', url: '/' }}
      generateLink={({ label, url, ...props }) => (
        <Link to={url} {...props}>{label}</Link>
      )}
      items={[{ label: 'New paste', url: '/' }]}
      itemsRight={
        isAuthenticated
          ? [
              { label: 'My pastes', url: '/my-pastes' },
              ...(userEmail ? [{ label: userEmail, url: '#' }] : []),
              { label: 'Log out', url: '/auth/logout' },
            ]
          : [{ label: 'Log in', url: '/auth/login' }]
      }
    />
  )
}
```

The `Navigation` component outputs Vanilla `p-navigation p-navigation--dark` markup with
the correct Aubergine header.

### 4.6 `web/src/pages/HomePage.tsx` / `MyPastesPage.tsx` / `PastePage.tsx`

All pages use the Vanilla layout shell:

```tsx
<>
  <Navigation isAuthenticated={isAuthenticated} />
  <main className="l-main">
    <section className="p-strip is-shallow">
      <div className="row">
        <div className="col-8">
          {/* page content */}
        </div>
      </div>
    </section>
  </main>
</>
```

- `HomePage` uses `col-8` for the paste creation form (narrower, form-focused).
- `MyPastesPage` uses `col-12` for the full-width table.
- `PastePage` uses `col-12` for the paste viewer.

### 4.7 `web/src/components/MyPastesList/MyPastesList.tsx`

Replace the `<ul className="p-list">` with `MainTable` from `@canonical/react-components`:

```tsx
import { MainTable } from '@canonical/react-components'

// columns
const headers = [
  { content: 'Title' },
  { content: 'Language' },
  { content: 'Created' },
]

// rows mapped from pastes
const rows = pastes.map((p) => ({
  key: p.key,
  columns: [
    { content: <Link to={`/${p.key}`}>{sanitizeTitle(p.title) || p.key}</Link> },
    { content: p.language },
    { content: new Date(p.created_at).toLocaleDateString() },
  ],
}))

return <MainTable headers={headers} rows={rows} />
```

### 4.8 `web/src/components/PasteViewer/PasteViewer.tsx`

- Replace all plain `<button>` elements with `<Button>` from `@canonical/react-components`.
- Use `appearance="base"` for secondary actions (raw, new paste, wrap, copy).
- Use `appearance="negative"` for Delete (already done).
- Render paste metadata in a `<table className="p-table">` or `<dl>` with Vanilla spacing.
- Keep `react-syntax-highlighter` for code display; style the wrapper with `u-no-margin`.

### 4.9 `web/src/components/NewPasteForm/NewPasteForm.tsx`

Form already uses Pragma components (Input, Select, Textarea, Button, Notification,
Spinner). No structural changes needed. Optionally add `className="p-form"` to the
`<form>` element for Vanilla form styling.

---

## 5. Vanilla Class Reference Used

| Purpose               | Class(es)                                     |
|-----------------------|-----------------------------------------------|
| App shell             | `l-application`                               |
| Main content area     | `l-main`                                      |
| Navigation            | Pragma `Navigation` component (dark theme)    |
| Content strip         | `p-strip is-shallow`                          |
| Grid row              | `row`                                         |
| Grid column (narrow)  | `col-8`                                       |
| Grid column (full)    | `col-12`                                      |
| Table                 | Pragma `MainTable` component                  |
| Form                  | `p-form`                                      |
| Buttons               | Pragma `Button` (`appearance="positive"`, etc.)|
| Notification          | Pragma `Notification`                         |

---

## 6. Typography

The Ubuntu font is loaded via `fonts.ubuntu.com`. Vanilla Framework's `$font-base` SCSS
variable defaults to `"Ubuntu, Arial, sans-serif"` when Vanilla detects the Ubuntu font
is available. No additional SCSS configuration is needed.

---

## 7. Acceptance Criteria

- [ ] App loads with the Ubuntu font in the browser.
- [ ] Top navigation bar is Aubergine/dark (Canonical brand colour).
- [ ] "My Pastes" renders as a `MainTable` with sortable columns.
- [ ] All interactive buttons use the Pragma `Button` component (no plain `<button>`).
- [ ] No custom CSS variables for colours remain in `index.css` or `App.css`.
- [ ] The grid uses `row` / `col-*` classes (no ad-hoc flexbox for layout).
- [ ] `npm run build` succeeds with no TypeScript errors.
- [ ] Existing unit and e2e tests continue to pass.
