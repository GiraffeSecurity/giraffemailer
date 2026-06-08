export interface NormalizedAttachment {
  filename: string
  content_type: string
  size_bytes: number
  part_path: string
}

function pickString(obj: Record<string, unknown>, keys: string[]): string {
  for (const k of keys) {
    const v = obj[k]
    if (typeof v === 'string' && v.trim()) return v.trim()
    if (typeof v === 'number' && Number.isFinite(v)) return String(v)
  }
  return ''
}

function pickNumber(obj: Record<string, unknown>, keys: string[]): number {
  for (const k of keys) {
    const v = obj[k]
    if (typeof v === 'number' && Number.isFinite(v)) return v
    if (typeof v === 'string' && v.trim()) {
      const n = Number(v)
      if (Number.isFinite(n)) return n
    }
  }
  return 0
}

export function normalizeAttachments(raw: unknown): NormalizedAttachment[] {
  if (!Array.isArray(raw)) return []
  return raw
    .map((item, index) => {
      if (!item || typeof item !== 'object') return null
      const att = item as Record<string, unknown>
      const part_path = pickString(att, ['part_path', 'partPath', 'PartPath']) || String(index + 1)
      const filename = pickString(att, ['filename', 'fileName', 'Filename'])
      return {
        part_path,
        filename: filename || `attachment-${part_path}`,
        content_type: pickString(att, ['content_type', 'contentType', 'ContentType']),
        size_bytes: pickNumber(att, ['size_bytes', 'sizeBytes', 'SizeBytes']),
      }
    })
    .filter((a): a is NormalizedAttachment => a !== null && !!a.part_path)
}

function parseContentDisposition(header: string | null): string {
  if (!header) return ''
  const star = header.match(/filename\*=(?:UTF-8''|utf-8'')([^;\n]+)/i)
  if (star?.[1]) {
    try {
      return decodeURIComponent(star[1].trim())
    } catch {
      return star[1].trim()
    }
  }
  const plain = header.match(/filename="([^"]+)"|filename=([^;\n]+)/i)
  return (plain?.[1] ?? plain?.[2] ?? '').trim()
}

import Cookies from 'js-cookie'
import { GM_API_URL } from '@/shared/config/env'

export async function downloadAttachment(
  msgId: string,
  partPath: string,
  filename?: string,
): Promise<boolean> {
  const path = partPath?.trim()
  if (!msgId || !path) return false

  const token = Cookies.get('gm_token')

  const res = await fetch(
    `${GM_API_URL}/api/v1/messages/${msgId}/attachments/${encodeURIComponent(path)}`,
    {
      credentials: 'include',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    },
  )
  if (!res.ok) return false

  const blob = await res.blob()
  let name = filename?.trim() || parseContentDisposition(res.headers.get('Content-Disposition'))
  if (!name) {
    const ext = blob.type.split('/')[1]?.split(';')[0] ?? 'bin'
    name = `attachment-${path}.${ext}`
  }

  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  return true
}
