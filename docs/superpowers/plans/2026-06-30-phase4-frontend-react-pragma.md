# Phase 4: Frontend (React + Pragma) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a React + TypeScript frontend in `web/` that lets users create, view, and manage pastes via the bingo Go API, styled with Canonical's Vanilla Framework via `@canonical/react-components`.

**Architecture:** Vite-bundled React SPA in `web/`. Component tests via Vitest + React Testing Library. E2E tests via Playwright with API mocking. In production, the Go server serves `web/dist/` as static files; in development, Vite's dev server proxies `/api` and `/auth` to the Go backend on port 8080.

**Tech Stack:** React 18, TypeScript (strict), Vite, `@canonical/react-components` + `vanilla-framework`, `react-syntax-highlighter`, `react-router-dom` v6, Vitest + `@testing-library/react`, Playwright.

## Global Constraints

- Node.js is installed via nvm at `$HOME/.nvm`. All npm/npx/node commands must source nvm first: `export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"`. The worktree is at `/home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4`.
- TypeScript `strict: true` in all tsconfigs.
- No `dangerouslySetInnerHTML` with untreated API content — ever. Paste content is user-supplied and rendered; all content strings must go through `sanitizeContent()` before use.
- No external session store; auth state is read from a CSRF cookie `csrf_token` (non-HttpOnly) set by `/auth/callback`. Frontend sends `X-CSRF-Token: <value>` header on all state-changing API calls.
- All `@canonical/react-components` components are used for UI — no raw `<input>`, `<button>`, etc. outside of tests.
- ESLint + Prettier — `npm run lint` must pass with 0 warnings before commit.
- Vitest unit tests + Playwright e2e tests — both suites must pass before merge.
- API base path: `/api/v1`. Auth routes: `/auth/login`, `/auth/callback`, `/auth/logout`.
- `expires_in` valid values: `"1d"`, `"1w"`, `"1mo"`, `"3mo"`, `"1y"`.
- `GET /api/v1/languages` returns `{ "languages": string[] }`.
- `POST /api/v1/pastes` returns `createResponse` (no `content` field). `GET /api/v1/pastes/{key}` returns `pasteResponse` (has `content`). `GET /api/v1/pastes?mine=true` returns `{ "pastes": pasteListItem[], "count": number }`.
- Go backend serves static files from `WEB_DIR` env var (default `web/dist`). When `WEB_DIR` is empty or unset, static file serving is disabled (Vite dev server handles it). Non-API/auth routes that don't match a static file fall back to `index.html` (SPA).
- React Router: `/` → NewPaste form; `/:key` → PasteViewer; `/my-pastes` → MyPastes (auth-required).
- Conventional commit messages (`feat:`, `fix:`, `chore:`).

---

## File Map

```
web/
├── index.html                          # Vite entry point
├── package.json
├── tsconfig.json
├── tsconfig.node.json
├── vite.config.ts                      # Vitest config embedded here; dev proxy to :8080
├── playwright.config.ts
├── .eslintrc.cjs
├── .prettierrc
├── src/
│   ├── main.tsx                        # ReactDOM.createRoot + BrowserRouter
│   ├── App.tsx                         # Routes (/, /:key, /my-pastes)
│   ├── api/
│   │   ├── types.ts                    # TypeScript interfaces for all API shapes
│   │   └── client.ts                   # Typed fetch wrappers; type guards; CSRF helpers
│   ├── utils/
│   │   └── sanitize.ts                 # sanitizeContent(), sanitizeTitle()
│   ├── components/
│   │   ├── NewPasteForm/
│   │   │   ├── NewPasteForm.tsx        # Form: title, language, expires_in, content
│   │   │   └── NewPasteForm.test.tsx
│   │   ├── PasteViewer/
│   │   │   ├── PasteViewer.tsx         # Syntax-highlighted view; copy; raw link
│   │   │   └── PasteViewer.test.tsx
│   │   ├── MyPastesList/
│   │   │   ├── MyPastesList.tsx        # Authenticated list of owned pastes
│   │   │   └── MyPastesList.test.tsx
│   │   └── Navigation/
│   │       ├── Navigation.tsx          # Top nav: logo, new paste, my pastes, login/logout
│   │       └── Navigation.test.tsx
│   ├── pages/
│   │   ├── HomePage.tsx                # Renders <NewPasteForm>
│   │   ├── PastePage.tsx               # Fetches paste by key; renders <PasteViewer>
│   │   └── MyPastesPage.tsx            # Auth-guard + renders <MyPastesList>
│   └── test/
│       └── setup.ts                    # @testing-library/jest-dom import
├── tests/
│   └── e2e/
│       ├── create-paste.spec.ts        # Playwright: fill form → 201 → viewer
│       ├── view-paste.spec.ts          # Playwright: navigate /{key} → content shown
│       └── navigation.spec.ts          # Playwright: nav links, new paste link
└── public/
    └── vite.svg                        # default Vite favicon (keep)
```

Go backend changes:
```
internal/
├── config/config.go                    # Add WebDir string field + WEB_DIR env var
└── server/server.go                    # Add serveStaticFiles(); call when WebDir != ""
```

---

### Task 1: Scaffold `web/` — Vite + TypeScript + all dependencies + toolchain config

**Files:**
- Create: `web/` (entire directory, via `npm create vite@latest`)
- Modify: `web/vite.config.ts` (add Vitest config + dev proxy)
- Create: `web/playwright.config.ts`
- Create: `web/.eslintrc.cjs`
- Create: `web/.prettierrc`
- Create: `web/src/test/setup.ts`
- Modify: `web/tsconfig.json` (add vitest types)

**Interfaces:**
- Consumes: nothing
- Produces: `npm test` runs Vitest (0 tests, no failures); `npm run build` produces `web/dist/`; `npm run lint` passes; `npm run test:e2e` exits 0 with 0 tests

- [ ] **Step 1: Scaffold with Vite React-TS template**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4
npm create vite@latest web -- --template react-ts
```

Expected: `web/` created with `src/App.tsx`, `src/main.tsx`, `index.html`, `package.json`, `vite.config.ts`, `tsconfig.json`.

- [ ] **Step 2: Install all dependencies**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd web

# Runtime deps
npm install @canonical/react-components vanilla-framework react-syntax-highlighter react-router-dom

# Dev deps: Vitest + RTL + Playwright
npm install -D \
  vitest \
  @vitest/coverage-v8 \
  @testing-library/react \
  @testing-library/user-event \
  @testing-library/jest-dom \
  jsdom \
  @playwright/test \
  @types/react-syntax-highlighter \
  prettier \
  eslint-config-prettier
```

- [ ] **Step 3: Install Playwright browsers**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npx playwright install --with-deps chromium
```

- [ ] **Step 4: Replace `web/vite.config.ts` with full config**

```typescript
/// <reference types="vitest" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/auth': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
    },
  },
})
```

- [ ] **Step 5: Create `web/src/test/setup.ts`**

```typescript
import '@testing-library/jest-dom'
```

- [ ] **Step 6: Update `web/tsconfig.json` — add Vitest global types and strict settings**

Replace the contents with:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "types": ["vitest/globals", "@testing-library/jest-dom"]
  },
  "include": ["src", "tests"]
}
```

- [ ] **Step 7: Create `web/playwright.config.ts`**

```typescript
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
```

- [ ] **Step 8: Create `web/.eslintrc.cjs`**

```javascript
module.exports = {
  root: true,
  env: { browser: true, es2020: true },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:react-hooks/recommended',
    'prettier',
  ],
  ignorePatterns: ['dist', '.eslintrc.cjs'],
  parser: '@typescript-eslint/parser',
  plugins: ['react-refresh'],
  rules: {
    'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
    'no-restricted-syntax': [
      'error',
      {
        selector: 'JSXAttribute[name.name="dangerouslySetInnerHTML"]',
        message: 'dangerouslySetInnerHTML is forbidden — use sanitizeContent() and JSX text nodes.',
      },
    ],
  },
}
```

Note: this ESLint config bans `dangerouslySetInnerHTML` at the lint level, enforcing §8.

- [ ] **Step 9: Create `web/.prettierrc`**

```json
{
  "semi": false,
  "singleQuote": true,
  "trailingComma": "es5",
  "printWidth": 100
}
```

- [ ] **Step 10: Update `web/package.json` scripts**

Add/replace the `"scripts"` block in `web/package.json`:

```json
{
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "lint": "eslint . --ext ts,tsx --report-unused-disable-directives --max-warnings 0",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "test:coverage": "vitest run --coverage",
    "test:e2e": "playwright test"
  }
}
```

- [ ] **Step 11: Verify the baseline works**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test 2>&1 | tail -5
```

Expected output contains: `Test Files  0 passed` or similar (no tests yet, no failures).

```bash
npm run build 2>&1 | tail -5
```

Expected: `dist/index.html` produced, no TypeScript errors.

- [ ] **Step 12: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4
git add web/
git commit -m "chore(web): scaffold Vite + React + TS + Vitest + Playwright"
```

---

### Task 2: API client — TypeScript types, fetch wrappers, and CSRF helpers

**Files:**
- Create: `web/src/api/types.ts`
- Create: `web/src/api/client.ts`
- Create: `web/src/api/client.test.ts`

**Interfaces:**
- Consumes: nothing from prior tasks
- Produces:
  - `createPaste(params: CreatePasteParams): Promise<CreatePasteResponse>` — `POST /api/v1/pastes`
  - `getPaste(key: string): Promise<PasteResponse>` — `GET /api/v1/pastes/{key}`
  - `deletePaste(key: string): Promise<void>` — `DELETE /api/v1/pastes/{key}`
  - `getLanguages(): Promise<string[]>` — `GET /api/v1/languages`
  - `getMyPastes(): Promise<MyPastesResponse>` — `GET /api/v1/pastes?mine=true`
  - `getCSRFToken(): string | null` — reads `csrf_token` cookie
  - Type guards: `isCreatePasteResponse`, `isPasteResponse`, `isMyPastesResponse`

- [ ] **Step 1: Create `web/src/api/types.ts`**

```typescript
// Exact mirror of backend JSON shapes (internal/server/handlers.go)

export interface CreatePasteParams {
  content: string
  language: string
  title?: string
  expires_in: '1d' | '1w' | '1mo' | '3mo' | '1y'
}

/** POST /api/v1/pastes → 201. No content field. */
export interface CreatePasteResponse {
  key: string
  url: string
  raw_url: string
  language: string
  title?: string
  size_bytes: number
  expires_at: string
  created_at: string
}

/** GET /api/v1/pastes/{key} → 200. Has content field. */
export interface PasteResponse extends Omit<CreatePasteResponse, 'raw_url'> {
  raw_url: string
  content: string
}

/** One item in GET /api/v1/pastes?mine=true response. */
export interface PasteListItem {
  key: string
  url: string
  language: string
  title?: string
  size_bytes: number
  expires_at: string
  created_at: string
}

/** GET /api/v1/pastes?mine=true → 200 */
export interface MyPastesResponse {
  pastes: PasteListItem[]
  count: number
}

/** Generic error envelope */
export interface ApiError {
  error: {
    code: string
    message: string
  }
}

export class ApiRequestError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string
  ) {
    super(message)
    this.name = 'ApiRequestError'
  }
}
```

- [ ] **Step 2: Write failing tests for the API client**

Create `web/src/api/client.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPaste, getPaste, deletePaste, getLanguages, getMyPastes, getCSRFToken } from './client'
import { ApiRequestError } from './types'

const mockFetch = vi.fn()
beforeEach(() => { vi.stubGlobal('fetch', mockFetch) })
afterEach(() => { vi.restoreAllMocks() })

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('getLanguages', () => {
  it('returns language array from API', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ languages: ['python', 'go', 'text'] }))
    const langs = await getLanguages()
    expect(langs).toEqual(['python', 'go', 'text'])
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/languages', expect.objectContaining({ method: 'GET' }))
  })
})

describe('getPaste', () => {
  it('returns paste response for valid key', async () => {
    const body = {
      key: 'abc12',
      url: 'http://localhost/abc12',
      raw_url: 'http://localhost/api/v1/pastes/abc12/raw',
      content: 'hello world',
      language: 'text',
      size_bytes: 11,
      expires_at: '2026-12-31T00:00:00Z',
      created_at: '2026-06-30T00:00:00Z',
    }
    mockFetch.mockResolvedValue(jsonResponse(body))
    const paste = await getPaste('abc12')
    expect(paste.key).toBe('abc12')
    expect(paste.content).toBe('hello world')
  })

  it('throws ApiRequestError with code paste_not_found on 404', async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ error: { code: 'paste_not_found', message: 'not found' } }, 404)
    )
    await expect(getPaste('nope')).rejects.toThrow(ApiRequestError)
    await expect(getPaste('nope')).rejects.toMatchObject({ status: 404, code: 'paste_not_found' })
  })
})

describe('createPaste', () => {
  it('sends POST with correct body and returns 201 response', async () => {
    const body = {
      key: 'xyz99',
      url: 'http://localhost/xyz99',
      raw_url: 'http://localhost/api/v1/pastes/xyz99/raw',
      language: 'python',
      size_bytes: 13,
      expires_at: '2026-12-31T00:00:00Z',
      created_at: '2026-06-30T00:00:00Z',
    }
    mockFetch.mockResolvedValue(jsonResponse(body, 201))
    const result = await createPaste({ content: 'print("hi")', language: 'python', expires_in: '1d' })
    expect(result.key).toBe('xyz99')
    const call = mockFetch.mock.calls[0]
    expect(call[0]).toBe('/api/v1/pastes')
    const options = call[1] as RequestInit
    expect(options.method).toBe('POST')
    expect(JSON.parse(options.body as string)).toMatchObject({ language: 'python' })
  })
})

describe('deletePaste', () => {
  it('sends DELETE and resolves on 204', async () => {
    mockFetch.mockResolvedValue(new Response(null, { status: 204 }))
    await expect(deletePaste('abc12')).resolves.toBeUndefined()
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/pastes/abc12', expect.objectContaining({ method: 'DELETE' }))
  })
})

describe('getMyPastes', () => {
  it('returns my-pastes response', async () => {
    const body = { pastes: [], count: 0 }
    mockFetch.mockResolvedValue(jsonResponse(body))
    const result = await getMyPastes()
    expect(result.count).toBe(0)
    expect(result.pastes).toEqual([])
  })
})

describe('getCSRFToken', () => {
  it('returns null when csrf_token cookie is absent', () => {
    Object.defineProperty(document, 'cookie', { value: '', configurable: true })
    expect(getCSRFToken()).toBeNull()
  })

  it('returns token value from csrf_token cookie', () => {
    Object.defineProperty(document, 'cookie', {
      value: 'csrf_token=abc123; other=x',
      configurable: true,
    })
    expect(getCSRFToken()).toBe('abc123')
  })
})
```

- [ ] **Step 3: Run tests to confirm they fail**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | tail -20
```

Expected: all tests fail with "Cannot find module './client'".

- [ ] **Step 4: Create `web/src/api/client.ts`**

```typescript
import {
  CreatePasteParams,
  CreatePasteResponse,
  PasteResponse,
  MyPastesResponse,
  ApiError,
  ApiRequestError,
} from './types'

// ─── CSRF ────────────────────────────────────────────────────────────────────

/** Reads the csrf_token non-HttpOnly cookie set by /auth/callback. */
export function getCSRFToken(): string | null {
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/)
  return match ? decodeURIComponent(match[1]) : null
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

async function request<T>(
  url: string,
  options: RequestInit = {},
  expectedStatus?: number
): Promise<T> {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  })
  const status = expectedStatus ?? 200
  if (res.status === (expectedStatus ?? res.status) && res.ok) {
    if (res.status === 204) return undefined as T
    return res.json() as Promise<T>
  }
  if (!res.ok) {
    let code = 'internal_error'
    let message = res.statusText
    try {
      const body = (await res.json()) as ApiError
      code = body.error?.code ?? code
      message = body.error?.message ?? message
    } catch {
      // ignore parse errors
    }
    throw new ApiRequestError(res.status, code, message)
  }
  return res.json() as Promise<T>
}

function csrfHeaders(): Record<string, string> {
  const token = getCSRFToken()
  return token ? { 'X-CSRF-Token': token } : {}
}

// ─── Public API ───────────────────────────────────────────────────────────────

export async function getLanguages(): Promise<string[]> {
  const body = await request<{ languages: string[] }>('/api/v1/languages', { method: 'GET' })
  return body.languages
}

export async function getPaste(key: string): Promise<PasteResponse> {
  return request<PasteResponse>(`/api/v1/pastes/${encodeURIComponent(key)}`, { method: 'GET' })
}

export async function createPaste(params: CreatePasteParams): Promise<CreatePasteResponse> {
  return request<CreatePasteResponse>('/api/v1/pastes', {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify(params),
  })
}

export async function deletePaste(key: string): Promise<void> {
  return request<void>(`/api/v1/pastes/${encodeURIComponent(key)}`, {
    method: 'DELETE',
    headers: csrfHeaders(),
  })
}

export async function getMyPastes(): Promise<MyPastesResponse> {
  return request<MyPastesResponse>('/api/v1/pastes?mine=true', { method: 'GET' })
}
```

- [ ] **Step 5: Run tests to confirm they pass**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | tail -20
```

Expected: all 8 tests pass.

- [ ] **Step 6: Run lint**

```bash
npm run lint 2>&1 | tail -5
```

Expected: 0 warnings, 0 errors.

- [ ] **Step 7: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4
git add web/src/api/
git commit -m "feat(web): add typed API client with CSRF support"
```

---

### Task 3: `NewPasteForm` component

**Files:**
- Create: `web/src/utils/sanitize.ts`
- Create: `web/src/utils/sanitize.test.ts`
- Create: `web/src/components/NewPasteForm/NewPasteForm.tsx`
- Create: `web/src/components/NewPasteForm/NewPasteForm.test.tsx`

**Interfaces:**
- Consumes: `createPaste`, `getLanguages` from `web/src/api/client.ts`; `CreatePasteParams` from `web/src/api/types.ts`
- Produces:
  - `sanitizeContent(raw: unknown): string` — strips null bytes/control chars, returns `''` if not a string
  - `sanitizeTitle(raw: unknown): string` — same + truncates to 255 chars
  - `<NewPasteForm onCreated={(key: string) => void} />` — fully controlled form component

- [ ] **Step 1: Create `web/src/utils/sanitize.ts`**

```typescript
/**
 * Validates and sanitizes a value expected to be a string.
 * Returns '' if the value is not a string.
 * Strips null bytes and non-printable ASCII control characters,
 * keeping whitespace (\t, \n, \r).
 */
export function sanitizeContent(raw: unknown): string {
  if (typeof raw !== 'string') return ''
  // Keep: 0x09=tab, 0x0A=newline, 0x0D=CR. Strip everything else below 0x20 and DEL.
  return raw.replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')
}

/**
 * Sanitizes a title string from an API response.
 * Returns '' if not a string. Truncates to 255 chars.
 */
export function sanitizeTitle(raw: unknown): string {
  if (typeof raw !== 'string') return ''
  return raw.replace(/[\x00-\x1F\x7F]/g, '').slice(0, 255)
}
```

- [ ] **Step 2: Write failing tests for sanitize utilities**

Create `web/src/utils/sanitize.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { sanitizeContent, sanitizeTitle } from './sanitize'

describe('sanitizeContent', () => {
  it('returns empty string for non-string input', () => {
    expect(sanitizeContent(null)).toBe('')
    expect(sanitizeContent(42)).toBe('')
    expect(sanitizeContent(undefined)).toBe('')
  })
  it('preserves normal text', () => {
    expect(sanitizeContent('hello world')).toBe('hello world')
  })
  it('preserves tab, newline, and carriage return', () => {
    expect(sanitizeContent('a\tb\nc\rd')).toBe('a\tb\nc\rd')
  })
  it('strips null bytes and control chars', () => {
    expect(sanitizeContent('a\x00b\x01c\x1Fd')).toBe('abcd')
  })
  it('strips DEL character', () => {
    expect(sanitizeContent('a\x7Fb')).toBe('ab')
  })
})

describe('sanitizeTitle', () => {
  it('returns empty string for non-string input', () => {
    expect(sanitizeTitle(null)).toBe('')
  })
  it('truncates to 255 chars', () => {
    expect(sanitizeTitle('a'.repeat(300))).toHaveLength(255)
  })
  it('strips all control characters including newline', () => {
    expect(sanitizeTitle('a\nb')).toBe('ab')
  })
})
```

- [ ] **Step 3: Run tests — confirm sanitize tests pass**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | tail -15
```

Expected: 8 prior client tests + 8 new sanitize tests pass.

- [ ] **Step 4: Write failing tests for `NewPasteForm`**

Create `web/src/components/NewPasteForm/NewPasteForm.test.tsx`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import * as client from '../../api/client'
import NewPasteForm from './NewPasteForm'

vi.mock('../../api/client')

beforeEach(() => {
  vi.mocked(client.getLanguages).mockResolvedValue(['text', 'python', 'go'])
  vi.mocked(client.createPaste).mockResolvedValue({
    key: 'abc12',
    url: 'http://localhost/abc12',
    raw_url: 'http://localhost/api/v1/pastes/abc12/raw',
    language: 'text',
    size_bytes: 5,
    expires_at: '2027-01-01T00:00:00Z',
    created_at: '2026-06-30T00:00:00Z',
  })
})

describe('NewPasteForm', () => {
  it('renders the content textarea', async () => {
    render(<NewPasteForm onCreated={() => {}} />)
    expect(screen.getByRole('textbox', { name: /content/i })).toBeInTheDocument()
  })

  it('loads languages into the language selector', async () => {
    render(<NewPasteForm onCreated={() => {}} />)
    await waitFor(() => expect(client.getLanguages).toHaveBeenCalled())
    expect(screen.getByRole('option', { name: 'python' })).toBeInTheDocument()
  })

  it('calls createPaste with form values on submit', async () => {
    const user = userEvent.setup()
    const onCreated = vi.fn()
    render(<NewPasteForm onCreated={onCreated} />)
    await waitFor(() => expect(client.getLanguages).toHaveBeenCalled())

    await user.type(screen.getByRole('textbox', { name: /content/i }), 'hello')
    await user.click(screen.getByRole('button', { name: /create paste/i }))

    await waitFor(() => expect(client.createPaste).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'hello' })
    ))
    expect(onCreated).toHaveBeenCalledWith('abc12')
  })

  it('shows an error message when createPaste fails', async () => {
    const user = userEvent.setup()
    vi.mocked(client.createPaste).mockRejectedValue(new Error('network error'))
    render(<NewPasteForm onCreated={() => {}} />)
    await user.type(screen.getByRole('textbox', { name: /content/i }), 'hello')
    await user.click(screen.getByRole('button', { name: /create paste/i }))
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
  })
})
```

- [ ] **Step 5: Run tests — confirm NewPasteForm tests fail**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | grep -E "FAIL|PASS|Error" | head -20
```

Expected: NewPasteForm tests fail with "Cannot find module './NewPasteForm'".

- [ ] **Step 6: Implement `web/src/components/NewPasteForm/NewPasteForm.tsx`**

```typescript
import React, { useEffect, useState } from 'react'
import { Button, Input, Select, Textarea, Notification, Spinner } from '@canonical/react-components'
import { createPaste, getLanguages } from '../../api/client'
import { CreatePasteParams } from '../../api/types'

interface Props {
  onCreated: (key: string) => void
}

const EXPIRY_OPTIONS = [
  { value: '1d', label: '1 day' },
  { value: '1w', label: '1 week' },
  { value: '1mo', label: '1 month' },
  { value: '3mo', label: '3 months' },
  { value: '1y', label: '1 year' },
] as const

export default function NewPasteForm({ onCreated }: Props) {
  const [languages, setLanguages] = useState<string[]>([])
  const [content, setContent] = useState('')
  const [title, setTitle] = useState('')
  const [language, setLanguage] = useState('text')
  const [expiresIn, setExpiresIn] = useState<CreatePasteParams['expires_in']>('1mo')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getLanguages()
      .then((langs) => {
        setLanguages(langs)
        if (langs.length > 0 && !langs.includes('text')) setLanguage(langs[0])
      })
      .catch(() => setLanguages(['text']))
  }, [])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!content.trim()) {
      setError('Content is required.')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const params: CreatePasteParams = { content, language, expires_in: expiresIn }
      if (title.trim()) params.title = title.trim()
      const resp = await createPaste(params)
      onCreated(resp.key)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create paste.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} aria-label="New paste form">
      {error && (
        <Notification severity="negative" title="Error" role="alert">
          {error}
        </Notification>
      )}
      <Input
        id="title"
        label="Title (optional)"
        type="text"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        maxLength={255}
      />
      <Select
        id="language"
        label="Syntax"
        value={language}
        onChange={(e) => setLanguage(e.target.value)}
        options={languages.map((l) => ({ value: l, label: l }))}
      />
      <Select
        id="expires_in"
        label="Expires in"
        value={expiresIn}
        onChange={(e) => setExpiresIn(e.target.value as CreatePasteParams['expires_in'])}
        options={EXPIRY_OPTIONS.map((o) => ({ value: o.value, label: o.label }))}
      />
      <Textarea
        id="content"
        label="Content"
        value={content}
        onChange={(e) => setContent(e.target.value)}
        rows={20}
        required
        aria-required="true"
      />
      <Button type="submit" appearance="positive" disabled={submitting}>
        {submitting ? <Spinner text="Creating…" /> : 'Create paste'}
      </Button>
    </form>
  )
}
```

- [ ] **Step 7: Run tests — confirm all pass**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | tail -20
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4
git add web/src/utils/ web/src/components/NewPasteForm/
git commit -m "feat(web): add sanitize utils and NewPasteForm component"
```

---

### Task 4: `PasteViewer` component — syntax highlighting and §8 security

**Files:**
- Create: `web/src/components/PasteViewer/PasteViewer.tsx`
- Create: `web/src/components/PasteViewer/PasteViewer.test.tsx`

**Interfaces:**
- Consumes:
  - `sanitizeContent(raw: unknown): string` from `web/src/utils/sanitize.ts`
  - `sanitizeTitle(raw: unknown): string` from `web/src/utils/sanitize.ts`
  - `PasteResponse` from `web/src/api/types.ts`
- Produces:
  - `<PasteViewer paste={PasteResponse} onDelete?: () => void />` — renders highlighted content

- [ ] **Step 1: Write failing tests for `PasteViewer`**

Create `web/src/components/PasteViewer/PasteViewer.test.tsx`:

```typescript
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import PasteViewer from './PasteViewer'
import { PasteResponse } from '../../api/types'

const basePaste: PasteResponse = {
  key: 'abc12',
  url: 'http://localhost/abc12',
  raw_url: 'http://localhost/api/v1/pastes/abc12/raw',
  content: 'print("hello")',
  language: 'python',
  size_bytes: 14,
  expires_at: '2027-01-01T00:00:00Z',
  created_at: '2026-06-30T00:00:00Z',
}

describe('PasteViewer', () => {
  it('renders paste content', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.getByText(/print/)).toBeInTheDocument()
  })

  it('renders language label', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.getByText(/python/i)).toBeInTheDocument()
  })

  it('renders a link to raw content', () => {
    render(<PasteViewer paste={basePaste} />)
    const rawLink = screen.getByRole('link', { name: /view raw/i })
    expect(rawLink).toHaveAttribute('href', basePaste.raw_url)
  })

  it('renders "New paste" link', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.getByRole('link', { name: /new paste/i })).toBeInTheDocument()
  })

  it('renders expiry date', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.getByText(/2027/)).toBeInTheDocument()
  })

  it('shows a title when present', () => {
    render(<PasteViewer paste={{ ...basePaste, title: 'My Script' }} />)
    expect(screen.getByText('My Script')).toBeInTheDocument()
  })

  it('calls onDelete when delete button is clicked', async () => {
    const user = userEvent.setup()
    const onDelete = vi.fn()
    render(<PasteViewer paste={basePaste} onDelete={onDelete} />)
    await user.click(screen.getByRole('button', { name: /delete/i }))
    expect(onDelete).toHaveBeenCalled()
  })

  it('does not render delete button when onDelete is not provided', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument()
  })

  it('renders a Copy button', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.getByRole('button', { name: /copy/i })).toBeInTheDocument()
  })

  it('calls navigator.clipboard.writeText with sanitized content when Copy clicked', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    render(<PasteViewer paste={basePaste} />)
    await user.click(screen.getByRole('button', { name: /copy/i }))
    expect(writeText).toHaveBeenCalledWith(basePaste.content)
  })

  it('sanitizes content before rendering (null byte stripped)', () => {
    render(<PasteViewer paste={{ ...basePaste, content: 'hel\x00lo' }} />)
    // content is sanitized — raw null byte does not appear
    expect(screen.queryByText(/\x00/)).not.toBeInTheDocument()
    expect(screen.getByText(/hello/)).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Run tests — confirm PasteViewer tests fail**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | grep -E "FAIL|Cannot find" | head -5
```

Expected: fail with "Cannot find module './PasteViewer'".

- [ ] **Step 3: Implement `web/src/components/PasteViewer/PasteViewer.tsx`**

```typescript
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
    return new Date(iso).toLocaleString()
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
          <header>
            {title && <h1>{title}</h1>}
            <dl>
              <dt>Language</dt>
              <dd>{paste.language}</dd>
              <dt>Created</dt>
              <dd>{formatDate(paste.created_at)}</dd>
              <dt>Expires</dt>
              <dd>{formatDate(paste.expires_at)}</dd>
              <dt>Size</dt>
              <dd>{paste.size_bytes} bytes</dd>
            </dl>
          </header>
          <div className="paste-actions">
            <a href={paste.raw_url} aria-label="View raw">View raw</a>
            {' · '}
            <a href="/" aria-label="New paste">New paste</a>
            {' · '}
            <button
              type="button"
              onClick={() => setWrapLines((w) => !w)}
              aria-pressed={wrapLines}
            >
              {wrapLines ? 'Unwrap' : 'Wrap'} lines
            </button>
            {' · '}
            <button
              type="button"
              aria-label="Copy to clipboard"
              onClick={() => navigator.clipboard.writeText(content)}
            >
              Copy
            </button>
            {onDelete && (
              <>
                {' · '}
                <Button
                  type="button"
                  appearance="negative"
                  small
                  onClick={onDelete}
                  aria-label="Delete paste"
                >
                  Delete
                </Button>
              </>
            )}
          </div>
          {/* §8: content passed as children (string) — SyntaxHighlighter does NOT use
              dangerouslySetInnerHTML for its own injected content; it renders tokens as
              React elements. We never pass untreated API strings to dangerouslySetInnerHTML. */}
          <SyntaxHighlighter
            language={paste.language}
            style={tomorrow}
            wrapLines={wrapLines}
            wrapLongLines={wrapLines}
            showLineNumbers
          >
            {content}
          </SyntaxHighlighter>
        </Col>
      </Row>
    </article>
  )
}
```

- [ ] **Step 4: Run tests — confirm all pass**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | tail -20
```

Expected: all tests pass (prior + 9 PasteViewer tests).

- [ ] **Step 5: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4
git add web/src/components/PasteViewer/
git commit -m "feat(web): add PasteViewer with syntax highlighting and §8 sanitization"
```

---

### Task 5: `MyPastesList` and `Navigation` components

**Files:**
- Create: `web/src/components/MyPastesList/MyPastesList.tsx`
- Create: `web/src/components/MyPastesList/MyPastesList.test.tsx`
- Create: `web/src/components/Navigation/Navigation.tsx`
- Create: `web/src/components/Navigation/Navigation.test.tsx`

**Interfaces:**
- Consumes:
  - `getMyPastes()` from `web/src/api/client.ts`
  - `PasteListItem`, `MyPastesResponse` from `web/src/api/types.ts`
  - `sanitizeTitle` from `web/src/utils/sanitize.ts`
- Produces:
  - `<MyPastesList />` — fetches + renders owned pastes with links
  - `<Navigation isAuthenticated: boolean; userEmail?: string />` — top nav

- [ ] **Step 1: Write failing tests for `MyPastesList`**

Create `web/src/components/MyPastesList/MyPastesList.test.tsx`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import * as client from '../../api/client'
import MyPastesList from './MyPastesList'

vi.mock('../../api/client')

const mockPastes = [
  {
    key: 'abc12',
    url: 'http://localhost/abc12',
    language: 'python',
    title: 'My script',
    size_bytes: 50,
    expires_at: '2027-01-01T00:00:00Z',
    created_at: '2026-06-30T00:00:00Z',
  },
]

beforeEach(() => {
  vi.mocked(client.getMyPastes).mockResolvedValue({ pastes: mockPastes, count: 1 })
})

describe('MyPastesList', () => {
  it('shows a loading state initially', () => {
    render(<MemoryRouter><MyPastesList /></MemoryRouter>)
    expect(screen.getByRole('status')).toBeInTheDocument()
  })

  it('renders paste titles after loading', async () => {
    render(<MemoryRouter><MyPastesList /></MemoryRouter>)
    await waitFor(() => expect(screen.getByText('My script')).toBeInTheDocument())
  })

  it('renders paste keys as links', async () => {
    render(<MemoryRouter><MyPastesList /></MemoryRouter>)
    await waitFor(() => {
      const link = screen.getByRole('link', { name: /abc12/i })
      expect(link).toHaveAttribute('href', '/abc12')
    })
  })

  it('shows empty state message when no pastes', async () => {
    vi.mocked(client.getMyPastes).mockResolvedValue({ pastes: [], count: 0 })
    render(<MemoryRouter><MyPastesList /></MemoryRouter>)
    await waitFor(() => expect(screen.getByText(/no pastes yet/i)).toBeInTheDocument())
  })

  it('shows error when API call fails', async () => {
    vi.mocked(client.getMyPastes).mockRejectedValue(new Error('fetch failed'))
    render(<MemoryRouter><MyPastesList /></MemoryRouter>)
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
  })
})
```

- [ ] **Step 2: Write failing tests for `Navigation`**

Create `web/src/components/Navigation/Navigation.test.tsx`:

```typescript
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Navigation from './Navigation'

describe('Navigation', () => {
  it('shows "New paste" link always', () => {
    render(<MemoryRouter><Navigation isAuthenticated={false} /></MemoryRouter>)
    expect(screen.getByRole('link', { name: /new paste/i })).toBeInTheDocument()
  })

  it('shows "Login" link when not authenticated', () => {
    render(<MemoryRouter><Navigation isAuthenticated={false} /></MemoryRouter>)
    expect(screen.getByRole('link', { name: /log in/i })).toBeInTheDocument()
  })

  it('shows "My pastes" and "Logout" when authenticated', () => {
    render(<MemoryRouter><Navigation isAuthenticated userEmail="a@b.com" /></MemoryRouter>)
    expect(screen.getByRole('link', { name: /my pastes/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /log out/i })).toBeInTheDocument()
  })

  it('shows user email when authenticated', () => {
    render(<MemoryRouter><Navigation isAuthenticated userEmail="a@b.com" /></MemoryRouter>)
    expect(screen.getByText('a@b.com')).toBeInTheDocument()
  })
})
```

- [ ] **Step 3: Run tests — confirm both sets fail**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | grep -E "FAIL|Cannot find" | head -10
```

Expected: both component modules not found.

- [ ] **Step 4: Implement `web/src/components/MyPastesList/MyPastesList.tsx`**

```typescript
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Notification, Spinner } from '@canonical/react-components'
import { getMyPastes } from '../../api/client'
import { PasteListItem } from '../../api/types'
import { sanitizeTitle } from '../../utils/sanitize'

export default function MyPastesList() {
  const [pastes, setPastes] = useState<PasteListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getMyPastes()
      .then((resp) => setPastes(resp.pastes))
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load pastes.'))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <Spinner role="status" text="Loading your pastes…" />
  if (error) return <Notification severity="negative" title="Error" role="alert">{error}</Notification>
  if (pastes.length === 0) return <p>No pastes yet. <Link to="/">Create one!</Link></p>

  return (
    <ul className="p-list">
      {pastes.map((p) => (
        <li key={p.key} className="p-list__item">
          <Link to={`/${p.key}`} aria-label={p.key}>
            {sanitizeTitle(p.title) || p.key}
          </Link>
          {' — '}
          <span>{p.language}</span>
          {' — '}
          <time dateTime={p.created_at}>{new Date(p.created_at).toLocaleDateString()}</time>
        </li>
      ))}
    </ul>
  )
}
```

- [ ] **Step 5: Implement `web/src/components/Navigation/Navigation.tsx`**

```typescript
import { Link } from 'react-router-dom'

interface Props {
  isAuthenticated: boolean
  userEmail?: string
}

export default function Navigation({ isAuthenticated, userEmail }: Props) {
  return (
    <nav className="p-navigation" aria-label="Main navigation">
      <div className="p-navigation__row">
        <div className="p-navigation__banner">
          <Link to="/" className="p-navigation__link">bingo</Link>
        </div>
        <ul className="p-navigation__items">
          <li className="p-navigation__item">
            <Link to="/" className="p-navigation__link" aria-label="New paste">New paste</Link>
          </li>
          {isAuthenticated ? (
            <>
              <li className="p-navigation__item">
                <Link to="/my-pastes" className="p-navigation__link" aria-label="My pastes">My pastes</Link>
              </li>
              {userEmail && (
                <li className="p-navigation__item">
                  <span className="p-navigation__link">{userEmail}</span>
                </li>
              )}
              <li className="p-navigation__item">
                <a href="/auth/logout" className="p-navigation__link" aria-label="Log out">Log out</a>
              </li>
            </>
          ) : (
            <li className="p-navigation__item">
              <a href="/auth/login" className="p-navigation__link" aria-label="Log in">Log in</a>
            </li>
          )}
        </ul>
      </div>
    </nav>
  )
}
```

- [ ] **Step 6: Run tests — confirm all pass**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | tail -25
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4
git add web/src/components/MyPastesList/ web/src/components/Navigation/
git commit -m "feat(web): add MyPastesList and Navigation components"
```

---

### Task 6: App shell — routing, pages, and Go static file serving

**Files:**
- Modify: `web/src/App.tsx`
- Modify: `web/src/main.tsx`
- Create: `web/src/pages/HomePage.tsx`
- Create: `web/src/pages/PastePage.tsx`
- Create: `web/src/pages/MyPastesPage.tsx`
- Modify: `internal/config/config.go` (add `WebDir`)
- Modify: `internal/server/server.go` (add `serveStaticFiles()`)
- Modify: `cmd/bingo/main.go` (pass `WebDir` to server when set)

**Interfaces:**
- Consumes:
  - `<NewPasteForm onCreated={(key) => navigate(`/${key}`)} />`
  - `<PasteViewer paste={PasteResponse} onDelete?={() => void} />`
  - `<MyPastesList />`
  - `<Navigation isAuthenticated userEmail />`
  - `getPaste(key)`, `deletePaste(key)` from api client
- Produces: `go test ./...` still passes; `npm run build` produces `web/dist/`; full SPA routing works

- [ ] **Step 1: Write the test for `PastePage` (the most interesting page)**

The `PastePage` fetches paste data and handles 404 — this is worth testing.

Create `web/src/pages/PastePage.test.tsx`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import * as client from '../api/client'
import PastePage from './PastePage'

vi.mock('../api/client')

const mockPaste = {
  key: 'abc12',
  url: 'http://localhost/abc12',
  raw_url: 'http://localhost/api/v1/pastes/abc12/raw',
  content: 'print("hi")',
  language: 'python',
  size_bytes: 11,
  expires_at: '2027-01-01T00:00:00Z',
  created_at: '2026-06-30T00:00:00Z',
}

beforeEach(() => {
  vi.mocked(client.getPaste).mockResolvedValue(mockPaste)
})

function renderPastePage(key = 'abc12') {
  return render(
    <MemoryRouter initialEntries={[`/${key}`]}>
      <Routes>
        <Route path="/:key" element={<PastePage />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('PastePage', () => {
  it('shows a loading spinner initially', () => {
    renderPastePage()
    expect(screen.getByRole('status')).toBeInTheDocument()
  })

  it('renders paste content after loading', async () => {
    renderPastePage()
    await waitFor(() => expect(screen.getByText(/print/)).toBeInTheDocument())
  })

  it('shows not-found message on 404', async () => {
    const { ApiRequestError } = await import('../api/types')
    vi.mocked(client.getPaste).mockRejectedValue(new ApiRequestError(404, 'paste_not_found', 'not found'))
    renderPastePage('nope')
    await waitFor(() => expect(screen.getByText(/not found/i)).toBeInTheDocument())
  })
})
```

- [ ] **Step 2: Run tests — confirm PastePage tests fail**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | grep -E "PastePage|FAIL" | head -5
```

Expected: fail with "Cannot find module './PastePage'".

- [ ] **Step 3: Replace `web/src/App.tsx`**

```typescript
import { Routes, Route } from 'react-router-dom'
import HomePage from './pages/HomePage'
import PastePage from './pages/PastePage'
import MyPastesPage from './pages/MyPastesPage'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
      <Route path="/my-pastes" element={<MyPastesPage />} />
      <Route path="/:key" element={<PastePage />} />
    </Routes>
  )
}
```

Note: `/my-pastes` is listed before `/:key` — React Router v6 matches static segments first, but explicit ordering is clearer.

- [ ] **Step 4: Replace `web/src/main.tsx`**

```typescript
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import 'vanilla-framework/build/css/vanilla.css'
import '@canonical/react-components/dist/index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </React.StrictMode>,
)
```

- [ ] **Step 5: Create `web/src/pages/HomePage.tsx`**

```typescript
import { useNavigate } from 'react-router-dom'
import Navigation from '../components/Navigation/Navigation'
import NewPasteForm from '../components/NewPasteForm/NewPasteForm'

export default function HomePage() {
  const navigate = useNavigate()
  // Read auth state from csrf_token cookie presence (non-HttpOnly, readable by JS)
  const isAuthenticated = document.cookie.includes('csrf_token=')

  return (
    <>
      <Navigation isAuthenticated={isAuthenticated} />
      <main className="l-main">
        <section className="p-strip">
          <div className="row">
            <NewPasteForm onCreated={(key) => navigate(`/${key}`)} />
          </div>
        </section>
      </main>
    </>
  )
}
```

- [ ] **Step 6: Create `web/src/pages/PastePage.tsx`**

```typescript
import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Spinner, Notification } from '@canonical/react-components'
import Navigation from '../components/Navigation/Navigation'
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
      <Navigation isAuthenticated={isAuthenticated} />
      <main className="l-main">
        <section className="p-strip">
          <div className="row">
            {!paste && !notFound && !error && <Spinner role="status" text="Loading…" />}
            {notFound && <p>Paste not found or has expired. <a href="/">Create a new paste.</a></p>}
            {error && <Notification severity="negative" title="Error">{error}</Notification>}
            {paste && <PasteViewer paste={paste} onDelete={isAuthenticated ? handleDelete : undefined} />}
          </div>
        </section>
      </main>
    </>
  )
}
```

- [ ] **Step 7: Create `web/src/pages/MyPastesPage.tsx`**

```typescript
import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import Navigation from '../components/Navigation/Navigation'
import MyPastesList from '../components/MyPastesList/MyPastesList'

export default function MyPastesPage() {
  const navigate = useNavigate()
  const isAuthenticated = document.cookie.includes('csrf_token=')

  useEffect(() => {
    // Redirect to home if not authenticated
    if (!isAuthenticated) navigate('/')
  }, [isAuthenticated, navigate])

  if (!isAuthenticated) return null

  return (
    <>
      <Navigation isAuthenticated userEmail={undefined} />
      <main className="l-main">
        <section className="p-strip">
          <div className="row">
            <h1>My pastes</h1>
            <MyPastesList />
          </div>
        </section>
      </main>
    </>
  )
}
```

- [ ] **Step 8: Run frontend tests — confirm all still pass**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test -- --reporter=verbose 2>&1 | tail -25
```

Expected: all tests including 3 new PastePage tests pass.

- [ ] **Step 9: Add `WebDir` config field to Go backend**

In `internal/config/config.go`, add `WebDir` to the `Config` struct and load it in `Load()`:

```go
// In Config struct, after existing fields:
WebDir string // WEB_DIR: path to web/dist; empty = disable static file serving
```

In `Load()`:
```go
cfg.WebDir = os.Getenv("WEB_DIR")
```

No validation needed — empty string disables static serving.

- [ ] **Step 10: Add `serveStaticFiles()` to Go server**

In `internal/server/server.go`:

First add imports:
```go
import (
    // existing imports...
    "net/http"
    "os"
    "path/filepath"
)
```

Add method after existing route setup (call it from `New()` when `cfg.WebDir != ""`):

```go
// serveStaticFiles registers a catch-all handler that serves web/dist as a SPA.
// Requests matching /api/* or /auth/* are not intercepted (already registered).
// Any path that doesn't resolve to an existing file falls back to index.html.
func (s *Server) serveStaticFiles(webDir string) {
    fs := http.FileServer(http.Dir(webDir))
    s.mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        path := filepath.Join(webDir, filepath.Clean("/"+r.URL.Path))
        _, err := os.Stat(path)
        if os.IsNotExist(err) {
            http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
            return
        }
        fs.ServeHTTP(w, r)
    }))
}
```

In `New()`, after route registration:
```go
if cfg.WebDir != "" {
    s.serveStaticFiles(cfg.WebDir)
}
```

- [ ] **Step 11: Verify frontend build does not emit unsafe-inline scripts (§8 CSP)**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
grep -r "unsafe-inline\|eval" dist/ 2>/dev/null && echo "WARNING: CSP violation found" || echo "OK: no unsafe-inline/eval in dist"
```

Expected: `OK: no unsafe-inline/eval in dist`. Vite's production build uses hash-based code splitting with no inline scripts. If this grep fires, investigate and do not proceed until clean.

- [ ] **Step 12: Verify Go tests still pass**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4
/home/daniel.nguyen@canonical.com/go/bin/go test ./... -count=1 2>&1
```

Expected: all 6 Go packages pass.

- [ ] **Step 12: Verify frontend build succeeds**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm run build 2>&1 | tail -10
```

Expected: `web/dist/index.html` produced, no TypeScript errors.

- [ ] **Step 13: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4
git add web/src/App.tsx web/src/main.tsx web/src/pages/ internal/config/config.go internal/server/server.go
git commit -m "feat(web,server): add SPA routing, pages, and Go static file serving"
```

---

### Task 7: Playwright E2E tests

**Files:**
- Create: `web/tests/e2e/create-paste.spec.ts`
- Create: `web/tests/e2e/view-paste.spec.ts`
- Create: `web/tests/e2e/navigation.spec.ts`

**Interfaces:**
- Consumes: the complete SPA from Tasks 1-6 (served by Vite dev server at localhost:5173); API mocked via `page.route()`
- Produces: `npm run test:e2e` passes all specs

Note: E2E tests use Playwright's `page.route()` to intercept API calls — no real backend needed. The Vite dev server serves the built assets.

- [ ] **Step 1: Create `web/tests/e2e/create-paste.spec.ts`**

```typescript
import { test, expect } from '@playwright/test'

test.describe('Create paste flow', () => {
  test.beforeEach(async ({ page }) => {
    // Mock GET /api/v1/languages
    await page.route('/api/v1/languages', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ languages: ['text', 'python', 'go'] }),
      })
    )
    // Mock POST /api/v1/pastes
    await page.route('/api/v1/pastes', async (route) => {
      if (route.request().method() === 'POST') {
        return route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            key: 'testk',
            url: 'http://localhost:5173/testk',
            raw_url: 'http://localhost:5173/api/v1/pastes/testk/raw',
            language: 'python',
            size_bytes: 12,
            expires_at: '2027-01-01T00:00:00Z',
            created_at: '2026-06-30T00:00:00Z',
          }),
        })
      }
      return route.continue()
    })
    // Mock GET /api/v1/pastes/testk (for redirect)
    await page.route('/api/v1/pastes/testk', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          key: 'testk',
          url: 'http://localhost:5173/testk',
          raw_url: 'http://localhost:5173/api/v1/pastes/testk/raw',
          content: 'print("hello")',
          language: 'python',
          size_bytes: 14,
          expires_at: '2027-01-01T00:00:00Z',
          created_at: '2026-06-30T00:00:00Z',
        }),
      })
    )
  })

  test('shows form on home page', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('textbox', { name: /content/i })).toBeVisible()
  })

  test('redirects to paste viewer after successful creation', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('textbox', { name: /content/i }).fill('print("hello")')
    await page.getByRole('button', { name: /create paste/i }).click()
    await expect(page).toHaveURL('/testk')
    await expect(page.getByText(/print/)).toBeVisible()
  })

  test('shows error notification when API returns 413', async ({ page }) => {
    await page.unroute('/api/v1/pastes')
    await page.route('/api/v1/pastes', (route) =>
      route.fulfill({
        status: 413,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'content_too_large', message: 'Paste exceeds limit.' } }),
      })
    )
    await page.goto('/')
    await page.getByRole('textbox', { name: /content/i }).fill('x')
    await page.getByRole('button', { name: /create paste/i }).click()
    await expect(page.getByRole('alert')).toBeVisible()
  })
})
```

- [ ] **Step 2: Create `web/tests/e2e/view-paste.spec.ts`**

```typescript
import { test, expect } from '@playwright/test'

test.describe('View paste', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('/api/v1/pastes/abc12', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          key: 'abc12',
          url: 'http://localhost:5173/abc12',
          raw_url: 'http://localhost:5173/api/v1/pastes/abc12/raw',
          content: 'def greet():\n    return "hello"',
          language: 'python',
          title: 'Greeting',
          size_bytes: 30,
          expires_at: '2027-01-01T00:00:00Z',
          created_at: '2026-06-30T00:00:00Z',
        }),
      })
    )
  })

  test('shows paste title and content', async ({ page }) => {
    await page.goto('/abc12')
    await expect(page.getByText('Greeting')).toBeVisible()
    await expect(page.getByText(/def greet/)).toBeVisible()
  })

  test('shows language label', async ({ page }) => {
    await page.goto('/abc12')
    await expect(page.getByText(/python/i)).toBeVisible()
  })

  test('shows View raw link pointing to raw URL', async ({ page }) => {
    await page.goto('/abc12')
    const rawLink = page.getByRole('link', { name: /view raw/i })
    await expect(rawLink).toBeVisible()
    await expect(rawLink).toHaveAttribute('href', /\/api\/v1\/pastes\/abc12\/raw/)
  })

  test('shows not-found message for expired/missing paste', async ({ page }) => {
    await page.route('/api/v1/pastes/nope', (route) =>
      route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'paste_not_found', message: 'not found' } }),
      })
    )
    await page.goto('/nope')
    await expect(page.getByText(/not found/i)).toBeVisible()
  })
})
```

- [ ] **Step 3: Create `web/tests/e2e/navigation.spec.ts`**

```typescript
import { test, expect } from '@playwright/test'

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('/api/v1/languages', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ languages: ['text'] }),
      })
    )
  })

  test('shows New paste link on home page', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('link', { name: /new paste/i })).toBeVisible()
  })

  test('shows Log in link when not authenticated', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('link', { name: /log in/i })).toBeVisible()
  })

  test('New paste link on viewer navigates to home', async ({ page }) => {
    await page.route('/api/v1/pastes/abc12', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          key: 'abc12',
          url: 'http://localhost:5173/abc12',
          raw_url: 'http://localhost:5173/api/v1/pastes/abc12/raw',
          content: 'hello',
          language: 'text',
          size_bytes: 5,
          expires_at: '2027-01-01T00:00:00Z',
          created_at: '2026-06-30T00:00:00Z',
        }),
      })
    )
    await page.goto('/abc12')
    await page.getByRole('link', { name: /new paste/i }).first().click()
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('textbox', { name: /content/i })).toBeVisible()
  })
})
```

- [ ] **Step 4: Update `playwright.config.ts` to use built assets served by Vite preview**

Replace `web/playwright.config.ts` with:

```typescript
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:4173',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'npm run preview',
    url: 'http://localhost:4173',
    reuseExistingServer: !process.env.CI,
    timeout: 30000,
  },
})
```

Using `vite preview` (port 4173) serves the production build — no live backend needed; all API calls are mocked by `page.route()`.

- [ ] **Step 5: Build and run E2E tests**

First build the frontend:
```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm run build 2>&1 | tail -5
```

Then run e2e tests:
```bash
npm run test:e2e -- --reporter=list 2>&1 | tail -25
```

Expected: all 11 Playwright tests pass. Any failures indicate a component or routing issue — fix before committing.

- [ ] **Step 6: Run full test suite one final time**

```bash
export NVM_DIR="$HOME/.nvm" && source "$NVM_DIR/nvm.sh"
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4/web
npm test 2>&1 | tail -10
npm run test:e2e -- --reporter=list 2>&1 | tail -10
npm run lint 2>&1 | tail -5
```

Expected: all unit tests pass, all e2e tests pass, 0 lint warnings.

Go tests:
```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4
/home/daniel.nguyen@canonical.com/go/bin/go test ./... -count=1 2>&1
```

Expected: all 6 Go packages pass.

- [ ] **Step 7: Commit**

```bash
cd /home/daniel.nguyen@canonical.com/bingo/.worktrees/bingo-phase4
git add web/tests/ web/playwright.config.ts
git commit -m "test(web): add Playwright E2E tests for create, view, and navigation"
```
