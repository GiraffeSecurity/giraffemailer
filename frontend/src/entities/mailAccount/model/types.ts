export interface MailAccount {
  id: string
  name: string
  email_address: string
  imap_host: string
  imap_port: number
  use_tls: boolean
  username: string
  sync_enabled: boolean
  last_sync_at: string | null
}

export interface CreateMailAccountRequest {
  name: string
  email_address: string
  imap_host: string
  imap_port: number
  use_tls: boolean
  username: string
  password: string
}
