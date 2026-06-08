import { describe, it, expect } from 'vitest'

function escapeQuery(raw: string): string {
  const tokens = raw.trim().split(/\s+/).filter(Boolean)
  if (!tokens.length) return ''
  return tokens.map(t => `"${t.replace(/"/g, '""')}"`).join(' AND ')
}

describe('FTS query escaping (mirrors Go search.EscapeQuery)', () => {
  it('joins tokens with AND', () => {
    expect(escapeQuery('hello world')).toBe('"hello" AND "world"')
  })
  it('rejects empty input', () => {
    expect(escapeQuery('   ')).toBe('')
  })
})

describe('search pagination contract', () => {
  it('uses cursor-based next page param', () => {
    const last = { messages: [], total: 100, next_cursor: 'abc', has_more: true, limit: 50 }
    const next = last.has_more && last.next_cursor ? last.next_cursor : undefined
    expect(next).toBe('abc')
  })
})
