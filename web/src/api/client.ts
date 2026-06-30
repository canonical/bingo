import {
  CreatePasteParams,
  CreatePasteResponse,
  PasteResponse,
  MyPastesResponse,
  ApiError,
  ApiRequestError,
} from './types'

// ─── CSRF ────────────────────────────────────────────────────────────────────

/** Reads the csrf_token non-HttpOnly cookie set by /auth/callback. */
export function getCSRFToken(): string | null {
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/)
  return match ? decodeURIComponent(match[1]) : null
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

async function request<T>(
  url: string,
  options: RequestInit = {}
): Promise<T> {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  })
  
  if (res.ok) {
    if (res.status === 204) return undefined as T
    return res.json() as Promise<T>
  }
  
  let code = 'internal_error'
  let message = res.statusText
  try {
    const body = (await res.json()) as ApiError
    code = body.error?.code ?? code
    message = body.error?.message ?? message
  } catch {
    // ignore parse errors
  }
  throw new ApiRequestError(res.status, code, message)
}

function csrfHeaders(): Record<string, string> {
  const token = getCSRFToken()
  return token ? { 'X-CSRF-Token': token } : {}
}

// ─── Public API ───────────────────────────────────────────────────────────────

export async function getLanguages(): Promise<string[]> {
  const body = await request<{ languages: string[] }>('/api/v1/languages', { method: 'GET' })
  return body.languages
}

export async function getPaste(key: string): Promise<PasteResponse> {
  return request<PasteResponse>(`/api/v1/pastes/${encodeURIComponent(key)}`, { method: 'GET' })
}

export async function createPaste(params: CreatePasteParams): Promise<CreatePasteResponse> {
  return request<CreatePasteResponse>('/api/v1/pastes', {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify(params),
  })
}

export async function deletePaste(key: string): Promise<void> {
  return request<void>(`/api/v1/pastes/${encodeURIComponent(key)}`, {
    method: 'DELETE',
    headers: csrfHeaders(),
  })
}

export async function getMyPastes(): Promise<MyPastesResponse> {
  return request<MyPastesResponse>('/api/v1/pastes?mine=true', { method: 'GET' })
}
