import { describe, it, expect } from 'vitest'
import { sanitizeContent, sanitizeTitle } from './sanitize'

describe('sanitizeContent', () => {
  it('returns empty string for non-string input', () => {
    expect(sanitizeContent(null)).toBe('')
    expect(sanitizeContent(42)).toBe('')
    expect(sanitizeContent(undefined)).toBe('')
  })
  it('preserves normal text', () => {
    expect(sanitizeContent('hello world')).toBe('hello world')
  })
  it('preserves tab, newline, and carriage return', () => {
    expect(sanitizeContent('a\tb\nc\rd')).toBe('a\tb\nc\rd')
  })
  it('strips null bytes and control chars', () => {
    expect(sanitizeContent('a\x00b\x01c\x1Fd')).toBe('abcd')
  })
  it('strips DEL character', () => {
    expect(sanitizeContent('a\x7Fb')).toBe('ab')
  })
})

describe('sanitizeTitle', () => {
  it('returns empty string for non-string input', () => {
    expect(sanitizeTitle(null)).toBe('')
  })
  it('truncates to 255 chars', () => {
    expect(sanitizeTitle('a'.repeat(300))).toHaveLength(255)
  })
  it('strips all control characters including newline', () => {
    expect(sanitizeTitle('a\nb')).toBe('ab')
  })
})
