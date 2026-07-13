import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import PasteViewer from './PasteViewer'
import { PasteResponse } from '../../api/types'

const basePaste: PasteResponse = {
  key: 'abc12',
  url: 'http://localhost/abc12',
  raw_url: 'http://localhost/api/v1/pastes/abc12/raw',
  content: 'print("hello")',
  language: 'python',
  size_bytes: 14,
  expires_at: '2027-01-01T00:00:00Z',
  created_at: '2026-06-30T00:00:00Z',
}

describe('PasteViewer', () => {
  it('renders paste content', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.getByText(/print/)).toBeInTheDocument()
  })

  it('renders language label', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.getByText(/python/i)).toBeInTheDocument()
  })

  it('renders a link to raw content', () => {
    render(<PasteViewer paste={basePaste} />)
    const rawLink = screen.getByRole('link', { name: /view raw/i })
    expect(rawLink).toHaveAttribute('href', basePaste.raw_url)
  })

  it('renders "New paste" link', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.getByRole('link', { name: /new paste/i })).toBeInTheDocument()
  })

  it('renders expiry date', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.getByText(/2027/)).toBeInTheDocument()
  })

  it('shows a title when present', () => {
    render(<PasteViewer paste={{ ...basePaste, title: 'My Script' }} />)
    expect(screen.getByText('My Script')).toBeInTheDocument()
  })

  it('calls onDelete when delete button is clicked', async () => {
    const user = userEvent.setup()
    const onDelete = vi.fn()
    render(<PasteViewer paste={basePaste} onDelete={onDelete} />)
    await user.click(screen.getByRole('button', { name: /delete/i }))
    expect(onDelete).toHaveBeenCalled()
  })

  it('does not render delete button when onDelete is not provided', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument()
  })

  it('renders a Copy button', () => {
    render(<PasteViewer paste={basePaste} />)
    expect(screen.getByRole('button', { name: /copy/i })).toBeInTheDocument()
  })

  it('calls navigator.clipboard.writeText with sanitized content when Copy clicked', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    render(<PasteViewer paste={basePaste} />)
    await user.click(screen.getByRole('button', { name: /copy/i }))
    expect(writeText).toHaveBeenCalledWith(basePaste.content)
  })

  it('shows "Copied!" feedback after a successful copy', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    render(<PasteViewer paste={basePaste} />)
    await user.click(screen.getByRole('button', { name: /copy to clipboard/i }))
    expect(await screen.findByText(/copied!/i)).toBeInTheDocument()
  })

  it('falls back to document.execCommand when the Clipboard API is unavailable (insecure context)', async () => {
    const user = userEvent.setup()
    Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true })
    const execCommand = vi.fn().mockReturnValue(true)
    document.execCommand = execCommand
    render(<PasteViewer paste={basePaste} />)
    await user.click(screen.getByRole('button', { name: /copy to clipboard/i }))
    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(await screen.findByText(/copied!/i)).toBeInTheDocument()
  })

  it('shows "Copy failed" feedback when both the Clipboard API and execCommand fallback fail', async () => {
    const user = userEvent.setup()
    Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true })
    document.execCommand = vi.fn().mockReturnValue(false)
    render(<PasteViewer paste={basePaste} />)
    await user.click(screen.getByRole('button', { name: /copy to clipboard/i }))
    expect(await screen.findByText(/copy failed/i)).toBeInTheDocument()
  })

  it('sanitizes content before rendering (null byte stripped)', () => {
    render(<PasteViewer paste={{ ...basePaste, content: 'hel\x00lo' }} />)
    // content is sanitized — raw null byte does not appear
    // eslint-disable-next-line no-control-regex
    expect(screen.queryByText(/\x00/)).not.toBeInTheDocument()
    expect(screen.getByText(/hello/)).toBeInTheDocument()
  })
})
