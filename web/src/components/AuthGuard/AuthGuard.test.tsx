import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import AuthGuard from './AuthGuard'
import * as useMe from '../../hooks/useMe'

describe('AuthGuard', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('shows the sign-in page when auth is enabled and the user is signed out', async () => {
    vi.spyOn(useMe, 'fetchMe').mockResolvedValue({ auth_enabled: true, authenticated: false })
    render(<MemoryRouter><AuthGuard><div>secret</div></AuthGuard></MemoryRouter>)
    expect(await screen.findByRole('link', { name: /sign in with oidc/i })).toHaveAttribute(
      'href',
      expect.stringContaining('/auth/login'),
    )
    expect(screen.queryByText('secret')).not.toBeInTheDocument()
  })

  it('renders children when authenticated', async () => {
    vi.spyOn(useMe, 'fetchMe').mockResolvedValue({ auth_enabled: true, authenticated: true, email: 'a@b.com' })
    render(<MemoryRouter><AuthGuard><div>secret</div></AuthGuard></MemoryRouter>)
    expect(await screen.findByText('secret')).toBeInTheDocument()
  })
})
