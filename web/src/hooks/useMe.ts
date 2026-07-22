import { useEffect, useState } from 'react'
import { getMe } from '../api/client'
import { MeResponse } from '../api/types'

const DEFAULT_ME: MeResponse = { auth_enabled: false, authenticated: false }

// Module-level cache so multiple components mounted on the same page share a
// single GET /api/v1/me request instead of each firing their own.
let cachedMe: MeResponse | null = null
let inFlight: Promise<MeResponse> | null = null

/**
 * Fetches GET /api/v1/me, caching the result module-wide. Rejections are
 * propagated (not swallowed here) so callers such as AuthGuard can inspect
 * the error — e.g. to distinguish a real HTTP error response from a network
 * failure. Note that a rejection is cached as-is: it is not retried on
 * subsequent calls within the same page load.
 */
export function fetchMe(): Promise<MeResponse> {
  if (cachedMe !== null) return Promise.resolve(cachedMe)
  if (!inFlight) {
    inFlight = getMe().then((me) => {
      cachedMe = me
      return me
    })
  }
  return inFlight
}

/**
 * Reports the current GET /api/v1/me state: whether OIDC auth is enabled on
 * the server, whether the user is authenticated, and their email if so.
 * Defaults to `{ auth_enabled: false, authenticated: false }` until the
 * request resolves (or if it fails), so no auth-only UI flashes before we
 * know it applies.
 */
export function useMe(): MeResponse {
  const [me, setMe] = useState(cachedMe ?? DEFAULT_ME)

  useEffect(() => {
    let cancelled = false
    fetchMe()
      .then((value) => {
        if (!cancelled) setMe(value)
      })
      .catch(() => {
        // AuthGuard is responsible for surfacing/handling this error
        // (redirecting to login or the error page); other consumers just
        // fall back to the default (unauthenticated) state.
      })
    return () => {
      cancelled = true
    }
  }, [])

  return me
}
