import { useEffect, useState } from 'react'
import { fetchMe } from '../../hooks/useMe'
import { ApiRequestError } from '../../api/types'
import { getBasePath } from '../../utils/basePath'
import SignInPage from '../../pages/SignInPage'

type AuthState = 'loading' | 'ready' | 'signed-out'

interface AuthGuardProps {
  children: React.ReactNode
}

/**
 * AuthGuard calls GET /api/v1/me on mount.
 * If auth is enabled and the user is not authenticated, shows a sign-in page
 * (rather than redirecting to the IdP, which would silently sign a user back
 * in right after they log out).
 * If the request fails with an HTTP error response, renders the app and lets
 * individual pages surface errors.
 * If the request fails due to a network error (no response reached the
 * server), redirects to /error rather than rendering a page that would
 * likely fail in confusing ways.
 * Renders nothing while loading to prevent a flash of unauthenticated content.
 */
export default function AuthGuard({ children }: AuthGuardProps) {
  const [state, setState] = useState<AuthState>('loading')

  useEffect(() => {
    fetchMe()
      .then((me) => {
        if (me.auth_enabled && !me.authenticated) {
          setState('signed-out')
        } else {
          setState('ready')
        }
      })
      .catch((err) => {
        if (err instanceof ApiRequestError) {
          // A real HTTP error response — render the app and let individual
          // pages surface errors.
          setState('ready')
        } else {
          // A true network failure (no response reached the server) —
          // rendering the underlying page would likely raise confusing
          // errors, so send the user to a static error page instead.
          window.location.href = `${getBasePath()}/error`
        }
      })
  }, [])

  if (state === 'loading') return null
  if (state === 'signed-out') return <SignInPage />

  return <>{children}</>
}
