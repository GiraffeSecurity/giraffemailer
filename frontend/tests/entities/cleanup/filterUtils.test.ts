import { describe, it, expect } from 'vitest'
import {
  parseCleanupFilter,
  formatFilterSummary,
  hasOptionalFilters,
  normalizeCleanupFilter,
  parseOptionalInt,
} from '@/entities/cleanup'
import type { CleanupJob } from '@/entities/cleanup'

describe('parseOptionalInt', () => {
  it('treats empty and zero as undefined', () => {
    expect(parseOptionalInt('')).toBeUndefined()
    expect(parseOptionalInt('0')).toBeUndefined()
  })
  it('parses positive integers', () => {
    expect(parseOptionalInt('30')).toBe(30)
  })
})

describe('normalizeCleanupFilter', () => {
  it('strips zero-age filters for today-inclusive cleanup', () => {
    const f = normalizeCleanupFilter({
      account_id: 'acc-1',
      older_than_days: 0,
      larger_than_kb: 0,
      sender_domain: ' spam.com ',
    })
    expect(f.older_than_days).toBeUndefined()
    expect(f.larger_than_kb).toBeUndefined()
    expect(f.sender_domain).toBe('spam.com')
  })
})

describe('formatFilterSummary', () => {
  it('shows all archived when no optional filters', () => {
    expect(formatFilterSummary({ account_id: 'x' })).toContain('All archived')
  })
})

describe('parseCleanupFilter', () => {
  it('parses stored job JSON and falls back to account_id column', () => {
    const job: CleanupJob = {
      id: 'j1',
      name: 'test',
      account_id: 'acc-1',
      filter: '{"account_id":"acc-1","older_than_days":7}',
      action: 'delete',
      move_target_folder: null,
      created_at: '',
    }
    const f = parseCleanupFilter(job)
    expect(f.account_id).toBe('acc-1')
    expect(f.older_than_days).toBe(7)
    expect(hasOptionalFilters(f)).toBe(true)
  })
})
