export interface Mailbox {
  id: string
  name: string
  message_count: number
  total_size_bytes: number
  archived_count: number
  archived_size_bytes: number
  archived_pct: number
  last_indexed_at: string | null
  last_archived_at: string | null
}

export interface Message {
  id: string
  uid: number
  subject: string | null
  sender_name: string | null
  sender_email: string
  date: string | null
  size_bytes: number
  has_attachments: boolean
  attachment_count: number
  body_preview: string | null
  archived_at: string | null
  deleted_from_server_at: string | null
  archive_state: 'not_archived' | 'archived' | 'deleted_from_server'
  mailbox_name?: string
  account_name?: string
}

export interface MessageDetail extends Message {
  body_html?: string
  body_text?: string
  attachments?: Array<{
    filename: string
    content_type: string
    size_bytes: number
    part_path: string
  }>
}

export interface MessageCursorResponse {
  messages: Message[]
  next_cursor: string
  has_more: boolean
  limit: number
}

export interface SearchResponse {
  messages: Message[]
  total: number
  next_cursor: string
  has_more: boolean
  limit: number
}

export interface InsightsData {
  total_messages: number
  archived_messages: number
  total_bytes: number
  archived_bytes: number
  reclaimable_bytes: number
  top_senders: Array<{ sender_email: string; count: number; total_bytes: number }>
  size_by_year: Array<{ year: string; count: number; total_bytes: number }>
}
