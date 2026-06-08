export interface CleanupJob {
  id: string
  name: string
  account_id: string
  filter: string
  action: 'delete' | 'move'
  move_target_folder: string | null
  created_at: string
}

export interface CleanupRun {
  id: string
  status: 'pending' | 'running' | 'done' | 'failed' | 'cancelled'
  total_candidates: number
  processed: number
  skipped_unarchived: number
  freed_bytes: number
  error_message: string | null
  started_at: string | null
  finished_at: string | null
}

export interface CleanupFilter {
  account_id: string
  mailbox_name?: string
  sender_domain?: string
  sender_email?: string
  older_than_days?: number
  larger_than_kb?: number
  has_attachments?: boolean
  flag_not_seen?: boolean
  subject_contains?: string
}

export interface CreateJobRequest {
  name: string
  filter: CleanupFilter
  action: 'delete' | 'move'
  move_target_folder?: string
}

export type UpdateJobRequest = CreateJobRequest

export interface PreviewResponse {
  count: number
  total_bytes: number
}
