import type { CleanupFilter, CleanupJob } from '../model/types'

export function parseCleanupFilter(job: CleanupJob): CleanupFilter {
  try {
    const f = JSON.parse(job.filter) as CleanupFilter
    if (!f.account_id) f.account_id = job.account_id
    return f
  } catch {
    return { account_id: job.account_id }
  }
}

export function hasOptionalFilters(filter: CleanupFilter): boolean {
  return !!(
    filter.mailbox_name ||
    filter.sender_domain ||
    filter.sender_email ||
    filter.older_than_days ||
    filter.larger_than_kb ||
    filter.has_attachments !== undefined ||
    filter.flag_not_seen ||
    filter.subject_contains
  )
}

export function formatFilterSummary(filter: CleanupFilter): string {
  if (!hasOptionalFilters(filter)) {
    return 'All archived mail on this account'
  }
  const parts: string[] = []
  if (filter.mailbox_name) parts.push(`mailbox: ${filter.mailbox_name}`)
  if (filter.sender_domain) parts.push(`domain: *@${filter.sender_domain}`)
  if (filter.sender_email) parts.push(`sender: ${filter.sender_email}`)
  if (filter.older_than_days) parts.push(`older than ${filter.older_than_days}d`)
  if (filter.larger_than_kb) parts.push(`≥ ${filter.larger_than_kb} KB`)
  if (filter.has_attachments === true) parts.push('with attachments')
  if (filter.has_attachments === false) parts.push('no attachments')
  if (filter.flag_not_seen) parts.push('unread only')
  if (filter.subject_contains) parts.push(`subject contains “${filter.subject_contains}”`)
  return parts.join(' · ')
}

export function emptyOptionalFilters(filter: CleanupFilter): CleanupFilter {
  return { account_id: filter.account_id }
}

export function normalizeCleanupFilter(filter: CleanupFilter): CleanupFilter {
  const f: CleanupFilter = { account_id: filter.account_id }
  if (filter.mailbox_name?.trim()) f.mailbox_name = filter.mailbox_name.trim()
  if (filter.sender_domain?.trim()) f.sender_domain = filter.sender_domain.trim()
  if (filter.sender_email?.trim()) f.sender_email = filter.sender_email.trim()
  if (filter.subject_contains?.trim()) f.subject_contains = filter.subject_contains.trim()
  if (filter.older_than_days && filter.older_than_days > 0) f.older_than_days = filter.older_than_days
  if (filter.larger_than_kb && filter.larger_than_kb > 0) f.larger_than_kb = filter.larger_than_kb
  if (filter.has_attachments !== undefined) f.has_attachments = filter.has_attachments
  if (filter.flag_not_seen) f.flag_not_seen = true
  return f
}

export function parseOptionalInt(raw: string): number | undefined {
  if (raw === '') return undefined
  const n = parseInt(raw, 10)
  if (Number.isNaN(n) || n <= 0) return undefined
  return n
}
