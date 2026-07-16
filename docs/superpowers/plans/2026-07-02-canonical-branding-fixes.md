# Canonical Branding Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the navigation banner to be full-width, replace the Vite/custom logo with the Canonical logo, and update the favicon to the Canonical COF mark.

**Architecture:** Three small, independent changes to `web/`: update `index.html` for the favicon, replace `public/favicon.svg` with a Canonical orange COF SVG, and update the `Navigation` component to use the white Canonical COF logo and the `fullWidth` prop.

**Tech Stack:** React, `@canonical/react-components` Navigation component, SVG, Vite dev server.

## Global Constraints

- Logo assets must be sourced from `https://assets.ubuntu.com/`
- Nav logo must be white/light (dark theme navigation bar)
- Favicon must be the orange Canonical COF mark
- Do NOT install new packages

---

### Task 1: Replace favicon and update `index.html`

**Files:**
- Modify: `web/public/favicon.svg` (replace content entirely)
- Modify: `web/index.html`

**Interfaces:**
- Produces: `/favicon.svg` serving the orange Canonical COF SVG; `<link rel="icon">` in `index.html` pointing to it

- [ ] **Step 1: Download the Canonical orange COF SVG into `public/favicon.svg`**

```bash
cd web
curl -sL "https://assets.ubuntu.com/v1/be6f4e50-canonical-logo.svg" -o public/favicon.svg
```

If that URL doesn't serve an SVG, fall back to inlining the standard Canonical COF:

```bash
cat > web/public/favicon.svg << 'EOF'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 60.45 57.87">
  <circle fill="#E95420" cx="9.66" cy="27.41" r="8.12"/>
  <circle fill="#E95420" cx="50.79" cy="7.58" r="8.12"/>
  <circle fill="#E95420" cx="50.79" cy="47.29" r="8.12"/>
  <path fill="#E95420" d="M35.1 32.5a8.12 8.12 0 0 1-5.77-13.88l-5.46-9.44a22.47 22.47 0 0 0 0 44.87l5.46-9.44A8.08 8.08 0 0 1 35.1 32.5z"/>
  <path fill="#E95420" d="M35.1 32.5a8.08 8.08 0 0 1 5.76-2.45l.01-10.92a22.43 22.43 0 0 0-11.23 3z"/>
  <path fill="#E95420" d="M35.1 32.5l-5.46 8.44a22.43 22.43 0 0 0 11.23 3l-.01-10.92A8.08 8.08 0 0 1 35.1 32.5z"/>
</svg>
EOF
```

- [ ] **Step 2: Update `index.html` to reference the favicon correctly**

Open `web/index.html`. The current `<link rel="icon">` should point to `/favicon.svg`. Change it to specify SVG type explicitly:

```html
<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
```

(It likely already says this — verify it matches exactly. If it already does, no change needed.)

- [ ] **Step 3: Verify the favicon renders in the browser**

Start `npm run dev` in `web/`, open `http://localhost:5173` and check the browser tab for the Canonical orange COF icon.

- [ ] **Step 4: Commit**

```bash
git add web/public/favicon.svg web/index.html
git commit -m "fix: replace favicon with Canonical COF mark

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```

---

### Task 2: Fix navigation — full-width banner and Canonical logo

**Files:**
- Modify: `web/src/components/Navigation/Navigation.tsx`

**Interfaces:**
- Consumes: `fullWidth` prop on `@canonical/react-components` `Navigation`
- Produces: nav bar spanning 100% page width; Canonical white COF logo replacing the current `/favicon.svg` logo src

- [ ] **Step 1: Update `Navigation.tsx`**

Replace the `<Navigation>` JSX with:

```tsx
  return (
    <Navigation
      fullWidth
      theme={Theme.DARK}
      logo={{
        src: 'https://assets.ubuntu.com/v1/82818827-CoF_white.svg',
        title: 'Bingo',
        url: '/',
      }}
      generateLink={generateLink}
      items={[{ label: 'New paste', url: '/' }]}
      itemsRight={rightItems}
    />
  )
```

Key changes:
- Added `fullWidth` prop (removes the inner `.row` max-width constraint)
- Changed `logo.src` from `'/favicon.svg'` to `'https://assets.ubuntu.com/v1/82818827-CoF_white.svg'` (white Canonical COF for dark nav)

- [ ] **Step 2: Verify in the browser**

With `npm run dev` running at `http://localhost:5173`:
- Navigation bar should span the full page width
- Canonical white circle-of-friends logo should appear on the left
- Nav links and login/logout should still appear on the right

- [ ] **Step 3: Run existing tests to confirm no regressions**

```bash
cd web && npm test
```

Expected: all tests pass (or pre-existing failures only — no new failures).

- [ ] **Step 4: Commit**

```bash
git add web/src/components/Navigation/Navigation.tsx
git commit -m "fix: use Canonical logo and full-width nav banner

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
```
