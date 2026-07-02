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
      const link = screen.getByRole('link', { name: /my script/i })
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
