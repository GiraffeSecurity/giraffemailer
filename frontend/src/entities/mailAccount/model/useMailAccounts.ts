'use client'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { mailAccountService } from '../api/mailAccountService'
import type { CreateMailAccountRequest } from './types'

export const MAIL_ACCOUNTS_KEY = ['mailAccounts'] as const

export function useMailAccounts() {
  return useQuery({
    queryKey: MAIL_ACCOUNTS_KEY,
    queryFn: () => mailAccountService.list(),
  })
}

export function useMailAccount(id: string) {
  return useQuery({
    queryKey: [...MAIL_ACCOUNTS_KEY, id],
    queryFn: () => mailAccountService.getById(id),
    enabled: !!id,
  })
}

export function useCreateMailAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateMailAccountRequest) => mailAccountService.create(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: MAIL_ACCOUNTS_KEY }),
  })
}

export function useDeleteMailAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => mailAccountService.remove(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: MAIL_ACCOUNTS_KEY }),
  })
}

export function useSyncMailAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => mailAccountService.sync(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: MAIL_ACCOUNTS_KEY }),
  })
}
