import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
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

  it('hides "Log in" link when auth is disabled', () => {
    render(<MemoryRouter><Navigation isAuthenticated={false} authEnabled={false} /></MemoryRouter>)
    expect(screen.queryByRole('link', { name: /log in/i })).not.toBeInTheDocument()
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

  it('logs out via a CSRF-protected POST', () => {
    Object.defineProperty(document, 'cookie', { value: 'csrf_token=tok123', configurable: true })
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 303 }))
    render(<MemoryRouter><Navigation isAuthenticated userEmail="a@b.com" /></MemoryRouter>)
    fireEvent.click(screen.getByRole('link', { name: /log out/i }))
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/auth/logout'),
      expect.objectContaining({ method: 'POST', headers: { 'X-CSRF-Token': 'tok123' } }),
    )
    fetchMock.mockRestore()
  })
})
