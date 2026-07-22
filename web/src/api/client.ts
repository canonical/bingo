import {
  CreatePasteParams,
  CreatePasteResponse,
  PasteResponse,
  MyPastesResponse,
  MeResponse,
  ApiError,
  ApiRequestError,
  isCreatePasteResponse,
  isPasteResponse,
  isMyPastesResponse,
  isMeResponse,
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
    ...options,
    headers: { 'Content-Type': 'application/json', ...options.headers },
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
  const body = await request<{ languages: string[] }>('api/v1/languages', { method: 'GET' })
  return body.languages
}

export async function getPaste(key: string): Promise<PasteResponse | null> {
  const data = await request<unknown>(`api/v1/pastes/${encodeURIComponent(key)}`, { method: 'GET' })
  if (data === undefined) return null  // 204 No Content — paste absent or expired
  if (!isPasteResponse(data)) throw new ApiRequestError(200, 'invalid_response', 'Unexpected response shape from GET /pastes/:key')
  return data
}

export async function createPaste(params: CreatePasteParams): Promise<CreatePasteResponse> {
  const data = await request<unknown>('api/v1/pastes', {
    method: 'POST',
    headers: csrfHeaders(),
    body: JSON.stringify(params),
  })
  if (!isCreatePasteResponse(data)) throw new ApiRequestError(201, 'invalid_response', 'Unexpected response shape from POST /pastes')
  return data
}

export async function deletePaste(key: string): Promise<void> {
  return request<void>(`api/v1/pastes/${encodeURIComponent(key)}`, {
    method: 'DELETE',
    headers: csrfHeaders(),
  })
}

export async function getMyPastes(): Promise<MyPastesResponse> {
  const data = await request<unknown>('api/v1/pastes?mine=true', { method: 'GET' })
  if (!isMyPastesResponse(data)) throw new ApiRequestError(200, 'invalid_response', 'Unexpected response shape from GET /pastes?mine=true')
  return data
}

export async function getMe(): Promise<MeResponse> {
  const data = await request<unknown>('api/v1/me', { method: 'GET' })
  if (!isMeResponse(data)) throw new ApiRequestError(200, 'invalid_response', 'Unexpected response shape from GET /me')
  return data
}
