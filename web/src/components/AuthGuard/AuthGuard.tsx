import { useEffect, useState } from 'react'
import { getMe } from '../../api/client'
import { getBasePath } from '../../utils/basePath'

type AuthState = 'loading' | 'ready'

interface AuthGuardProps {
  children: React.ReactNode
}

/**
 * AuthGuard calls GET /api/v1/me on mount.
 * If auth is enabled and the user is not authenticated, redirects to /auth/login.
 * Renders nothing while loading to prevent a flash of unauthenticated content.
 */
export default function AuthGuard({ children }: AuthGuardProps) {
  const [state, setState] = useState<AuthState>('loading')

  useEffect(() => {
    getMe()
      .then((me) => {
        if (me.auth_enabled && !me.authenticated) {
          window.location.href = `${getBasePath()}/auth/login`
        } else {
          setState('ready')
        }
      })
      .catch(() => {
        // Network error — render the app and let individual pages surface errors.
        setState('ready')
      })
  }, [])

  if (state === 'loading') return null

  return <>{children}</>
}
