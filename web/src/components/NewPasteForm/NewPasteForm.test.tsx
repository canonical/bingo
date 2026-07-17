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
