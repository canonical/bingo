import { useEffect, useState } from 'react'
import { getMe } from '../api/client'

// Module-level cache so multiple components mounted on the same page share a
// single GET /api/v1/me request instead of each firing their own.
let cachedAuthEnabled: boolean | null = null
let inFlight: Promise<boolean> | null = null

function fetchAuthEnabled(): Promise<boolean> {
  if (cachedAuthEnabled !== null) return Promise.resolve(cachedAuthEnabled)
  if (!inFlight) {
    inFlight = getMe()
      .then((me) => {
        cachedAuthEnabled = me.auth_enabled
        return cachedAuthEnabled
      })
      .catch(() => false)
  }
  return inFlight
}

/**
 * Reports whether OIDC authentication is enabled on the server (GET
 * /api/v1/me → auth_enabled). Used to hide auth-only UI, such as the
 * "Log in" nav link, when no auth provider is configured.
 * Defaults to `false` until the request resolves, so no such UI flashes
 * before we know it applies.
 */
export function useAuthEnabled(): boolean {
  const [authEnabled, setAuthEnabled] = useState(cachedAuthEnabled ?? false)

  useEffect(() => {
    let cancelled = false
    fetchAuthEnabled().then((value) => {
      if (!cancelled) setAuthEnabled(value)
    })
    return () => {
      cancelled = true
    }
  }, [])

  return authEnabled
}
