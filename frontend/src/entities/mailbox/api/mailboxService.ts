import GmHttpService from '@/shared/api/gmHttpService'
import type {
  Mailbox,
  MessageCursorResponse,
  MessageDetail,
  SearchResponse,
  InsightsData,
} from '../model/types'

class MailboxService extends GmHttpService {
  listMailboxes(accountId: string) {
    return this.get<Mailbox[]>(`/api/v1/accounts/${accountId}/mailboxes`)
  }

  listMessages(
    accountId: string,
    mailboxId: string,
    params?: Record<string, unknown>,
  ) {
    return this.get<MessageCursorResponse>(
      `/api/v1/accounts/${accountId}/mailboxes/${mailboxId}/messages`,
      params,
    )
  }

  listAllMessages(params?: Record<string, unknown>) {
    return this.get<MessageCursorResponse>('/api/v1/messages', params)
  }

  getMessage(id: string, loadImages?: boolean) {
    return this.get<MessageDetail>(`/api/v1/messages/${id}`, {
      ...(loadImages ? { load_images: 'true' } : {}),
    })
  }

  search(params: Record<string, unknown>) {
    return this.get<SearchResponse>('/api/v1/search', params)
  }

  insights() {
    return this.get<InsightsData>('/api/v1/insights')
  }

  sync(id: string) {
    return this.post(`/api/v1/accounts/${id}/sync`, {})
  }
}

export const mailboxService = new MailboxService()
