/**
 * getBasePath returns the app's externally-visible base path, as reflected
 * by the <base href> tag the server injects into index.html (see
 * internal/server/server.go's indexHTMLWithBase). Returns "" when the app
 * is served from the domain root (the tag's href is "/"), so callers can
 * concatenate it directly with a leading-slash path, e.g.
 * `${getBasePath()}/auth/login`.
 *
 * This is needed for the handful of places that navigate via an absolute
 * URL (a real browser navigation, not React Router) — router-based
 * navigation (<Link>, useNavigate) already resolves correctly via
 * BrowserRouter's `basename` prop (see main.tsx), which is set from this
 * same value.
 */
export function getBasePath(): string {
  const href = document.querySelector('base')?.getAttribute('href') ?? '/'
  return href.replace(/\/$/, '')
}
