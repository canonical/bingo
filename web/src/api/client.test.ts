import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPaste, getPaste, deletePaste, getLanguages, getMyPastes, getCSRFToken } from './client'
import { ApiRequestError, isCreatePasteResponse, isPasteResponse, isMyPastesResponse } from './types'

const mockFetch = vi.fn()
beforeEach(() => { 
  vi.stubGlobal('fetch', mockFetch)
  mockFetch.mockClear()
})
afterEach(() => { vi.restoreAllMocks() })

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('getLanguages', () => {
  it('returns language array from API', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ languages: ['python', 'go', 'text'] }))
    const langs = await getLanguages()
    expect(langs).toEqual(['python', 'go', 'text'])
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/languages', expect.objectContaining({ method: 'GET' }))
  })
})

describe('getPaste', () => {
  it('returns paste response for valid key', async () => {
    const body = {
      key: 'abc12',
      url: 'http://localhost/abc12',
      raw_url: 'http://localhost/api/v1/pastes/abc12/raw',
      content: 'hello world',
      language: 'text',
      size_bytes: 11,
      expires_at: '2026-12-31T00:00:00Z',
      created_at: '2026-06-30T00:00:00Z',
    }
    mockFetch.mockResolvedValue(jsonResponse(body))
    const paste = await getPaste('abc12')
    expect(paste.key).toBe('abc12')
    expect(paste.content).toBe('hello world')
  })

  it('throws ApiRequestError with code paste_not_found on 404', async () => {
    const response = jsonResponse({ error: { code: 'paste_not_found', message: 'not found' } }, 404)
    mockFetch.mockResolvedValue(response)
    try {
      await getPaste('nope')
      throw new Error('Should have thrown')
    } catch (e) {
      expect(e).toBeInstanceOf(ApiRequestError)
      expect((e as ApiRequestError).status).toBe(404)
      expect((e as ApiRequestError).code).toBe('paste_not_found')
    }
  })
})

describe('createPaste', () => {
  it('sends POST with correct body and returns 201 response', async () => {
    const body = {
      key: 'xyz99',
      url: 'http://localhost/xyz99',
      raw_url: 'http://localhost/api/v1/pastes/xyz99/raw',
      language: 'python',
      size_bytes: 13,
      expires_at: '2026-12-31T00:00:00Z',
      created_at: '2026-06-30T00:00:00Z',
    }
    mockFetch.mockResolvedValue(jsonResponse(body, 201))
    const result = await createPaste({ content: 'print("hi")', language: 'python', expires_in: '1d' })
    expect(result.key).toBe('xyz99')
    const call = mockFetch.mock.calls[0]
    expect(call[0]).toBe('/api/v1/pastes')
    const options = call[1] as RequestInit
    expect(options.method).toBe('POST')
    expect(JSON.parse(options.body as string)).toMatchObject({ language: 'python' })
  })
})

describe('deletePaste', () => {
  it('sends DELETE and resolves on 204', async () => {
    mockFetch.mockResolvedValue(new Response(null, { status: 204 }))
    await expect(deletePaste('abc12')).resolves.toBeUndefined()
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/pastes/abc12', expect.objectContaining({ method: 'DELETE' }))
  })
})

describe('getMyPastes', () => {
  it('returns my-pastes response', async () => {
    const body = { pastes: [], count: 0 }
    mockFetch.mockResolvedValue(jsonResponse(body))
    const result = await getMyPastes()
    expect(result.count).toBe(0)
    expect(result.pastes).toEqual([])
  })
})

describe('getCSRFToken', () => {
  it('returns null when csrf_token cookie is absent', () => {
    Object.defineProperty(document, 'cookie', { value: '', configurable: true })
    expect(getCSRFToken()).toBeNull()
  })

  it('returns token value from csrf_token cookie', () => {
    Object.defineProperty(document, 'cookie', {
      value: 'csrf_token=abc123; other=x',
      configurable: true,
    })
    expect(getCSRFToken()).toBe('abc123')
  })
})

describe('type guards', () => {
  it('isCreatePasteResponse: true for valid object without content', () => {
    const obj = { key: 'a', url: 'u', raw_url: 'r', language: 'text', size_bytes: 1, expires_at: 'e', created_at: 'c' }
    expect(isCreatePasteResponse(obj)).toBe(true)
  })
  it('isCreatePasteResponse: false when content present', () => {
    expect(isCreatePasteResponse({ key: 'a', url: 'u', raw_url: 'r', language: 'text', content: 'x', size_bytes: 1, expires_at: 'e', created_at: 'c' })).toBe(false)
  })
  it('isPasteResponse: true for valid object with content', () => {
    expect(isPasteResponse({ key: 'a', url: 'u', raw_url: 'r', content: 'hi', language: 'text', size_bytes: 2, expires_at: 'e', created_at: 'c' })).toBe(true)
  })
  it('isMyPastesResponse: true for { pastes: [], count: 0 }', () => {
    expect(isMyPastesResponse({ pastes: [], count: 0 })).toBe(true)
  })
  it('isMyPastesResponse: false for non-object', () => {
    expect(isMyPastesResponse(null)).toBe(false)
  })
})
