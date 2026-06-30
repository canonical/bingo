// Exact mirror of backend JSON shapes (internal/server/handlers.go)

export interface CreatePasteParams {
  content: string
  language: string
  title?: string
  expires_in: '1d' | '1w' | '1mo' | '3mo' | '1y'
}

/** POST /api/v1/pastes → 201. No content field. */
export interface CreatePasteResponse {
  key: string
  url: string
  raw_url: string
  language: string
  title?: string
  size_bytes: number
  expires_at: string
  created_at: string
}

/** GET /api/v1/pastes/{key} → 200. Has content field. */
export interface PasteResponse extends Omit<CreatePasteResponse, 'raw_url'> {
  raw_url: string
  content: string
}

/** One item in GET /api/v1/pastes?mine=true response. */
export interface PasteListItem {
  key: string
  url: string
  language: string
  title?: string
  size_bytes: number
  expires_at: string
  created_at: string
}

/** GET /api/v1/pastes?mine=true → 200 */
export interface MyPastesResponse {
  pastes: PasteListItem[]
  count: number
}

/** Generic error envelope */
export interface ApiError {
  error: {
    code: string
    message: string
  }
}

export class ApiRequestError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string
  ) {
    super(message)
    this.name = 'ApiRequestError'
  }
}

export function isCreatePasteResponse(obj: unknown): obj is CreatePasteResponse {
  if (typeof obj !== 'object' || obj === null) return false
  const o = obj as Record<string, unknown>
  return (
    typeof o.key === 'string' &&
    typeof o.url === 'string' &&
    typeof o.raw_url === 'string' &&
    typeof o.language === 'string' &&
    typeof o.size_bytes === 'number' &&
    typeof o.expires_at === 'string' &&
    typeof o.created_at === 'string' &&
    !('content' in o)
  )
}

export function isPasteResponse(obj: unknown): obj is PasteResponse {
  if (typeof obj !== 'object' || obj === null) return false
  const o = obj as Record<string, unknown>
  return (
    typeof o.key === 'string' &&
    typeof o.url === 'string' &&
    typeof o.raw_url === 'string' &&
    typeof o.content === 'string' &&
    typeof o.language === 'string' &&
    typeof o.size_bytes === 'number' &&
    typeof o.expires_at === 'string' &&
    typeof o.created_at === 'string'
  )
}

export function isMyPastesResponse(obj: unknown): obj is MyPastesResponse {
  if (typeof obj !== 'object' || obj === null) return false
  const o = obj as Record<string, unknown>
  return Array.isArray(o.pastes) && typeof o.count === 'number'
}
