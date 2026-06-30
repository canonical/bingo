/**
 * Validates and sanitizes a value expected to be a string.
 * Returns '' if the value is not a string.
 * Strips null bytes and non-printable ASCII control characters,
 * keeping whitespace (\t, \n, \r).
 */
export function sanitizeContent(raw: unknown): string {
  if (typeof raw !== 'string') return ''
  // Keep: 0x09=tab, 0x0A=newline, 0x0D=CR. Strip everything else below 0x20 and DEL.
  return raw.replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '')
}

/**
 * Sanitizes a title string from an API response.
 * Returns '' if not a string. Truncates to 255 chars.
 */
export function sanitizeTitle(raw: unknown): string {
  if (typeof raw !== 'string') return ''
  return raw.replace(/[\x00-\x1F\x7F]/g, '').slice(0, 255)
}
