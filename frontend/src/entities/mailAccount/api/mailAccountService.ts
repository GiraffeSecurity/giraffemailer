import GmHttpService from '@/shared/api/gmHttpService'
import type { MailAccount, CreateMailAccountRequest } from '../model/types'

class MailAccountService extends GmHttpService {
  list() {
    return this.get<MailAccount[]>('/api/v1/accounts')
  }

  getById(id: string) {
    return this.get<MailAccount>(`/api/v1/accounts/${id}`)
  }

  create(data: CreateMailAccountRequest) {
    return this.post<{ id: string }>('/api/v1/accounts', data)
  }

  remove(id: string) {
    return this.delete<{ message: string }>(`/api/v1/accounts/${id}`)
  }

  testConnection(id: string) {
    return this.post<{ success: boolean; error?: string }>(`/api/v1/accounts/${id}/test`)
  }

  sync(id: string) {
    return this.post<{ status: string }>(`/api/v1/accounts/${id}/sync`)
  }
}

export const mailAccountService = new MailAccountService()
