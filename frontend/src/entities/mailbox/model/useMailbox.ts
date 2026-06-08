'use client'
import { useQuery, useInfiniteQuery } from '@tanstack/react-query'
import { mailboxService } from '../api/mailboxService'

export const MAILBOXES_KEY = (accountId: string) => ['mailboxes', accountId] as const
export const MESSAGES_KEY = (accountId: string, mailboxId: string, params?: unknown) =>
  ['messages', accountId, mailboxId, params] as const
export const INF_MESSAGES_KEY = (accountId: string, mailboxId: string, params?: unknown) =>
  ['infMessages', accountId, mailboxId, params] as const
export const MESSAGE_KEY = (id: string) => ['message', id] as const
export const SEARCH_KEY = (params: unknown) => ['search', params] as const
export const INSIGHTS_KEY = ['insights'] as const
export const ALL_MESSAGES_KEY = (params?: unknown) => ['allMessages', params] as const

export function useMailboxes(accountId: string) {
  return useQuery({
    queryKey: MAILBOXES_KEY(accountId),
    queryFn: () => mailboxService.listMailboxes(accountId),
    enabled: !!accountId,
  })
}

export function useMessages(
  accountId: string,
  mailboxId: string,
  params?: Record<string, unknown>,
) {
  return useQuery({
    queryKey: MESSAGES_KEY(accountId, mailboxId, params),
    queryFn: () => mailboxService.listMessages(accountId, mailboxId, params),
    enabled: !!accountId && !!mailboxId,
  })
}

export function useInfiniteMessages(
  accountId: string,
  mailboxId: string,
  params?: Record<string, unknown>,
) {
  return useInfiniteQuery({
    queryKey: INF_MESSAGES_KEY(accountId, mailboxId, params),
    queryFn: ({ pageParam }) =>
      mailboxService.listMessages(accountId, mailboxId, { ...params, cursor: pageParam || undefined }),
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
    enabled: !!accountId && !!mailboxId,
  })
}

export function useAllMessages(params?: Record<string, unknown>) {
  return useInfiniteQuery({
    queryKey: ALL_MESSAGES_KEY(params),
    queryFn: ({ pageParam }) =>
      mailboxService.listAllMessages({ ...params, cursor: pageParam || undefined }),
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
    enabled: params !== undefined,
  })
}

export function useMessage(id: string) {
  return useQuery({
    queryKey: MESSAGE_KEY(id),
    queryFn: () => mailboxService.getMessage(id),
    enabled: !!id,
  })
}

export function useSearch(params: Record<string, unknown>, enabled: boolean) {
  return useQuery({
    queryKey: SEARCH_KEY(params),
    queryFn: () => mailboxService.search(params),
    enabled,
  })
}

export function useInfiniteSearch(
  params: Record<string, unknown>,
  enabled: boolean,
) {
  const { cursor: _cursor, page: _page, ...base } = params
  return useInfiniteQuery({
    queryKey: SEARCH_KEY(base),
    queryFn: ({ pageParam }) =>
      mailboxService.search({ ...base, cursor: pageParam || undefined }),
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
    enabled: enabled && !!base.q,
  })
}

export function useInsights() {
  return useQuery({
    queryKey: INSIGHTS_KEY,
    queryFn: () => mailboxService.insights(),
  })
}
