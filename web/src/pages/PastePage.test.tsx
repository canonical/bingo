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
  vi.mocked(client.getMe).mockResolvedValue({ auth_enabled: false, authenticated: false })
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
