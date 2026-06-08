export type {
  Mailbox,
  Message,
  MessageDetail,
  MessageCursorResponse,
  SearchResponse,
  InsightsData,
} from './model/types'
export { mailboxService } from './api/mailboxService'
export {
  MAILBOXES_KEY,
  MESSAGES_KEY,
  INF_MESSAGES_KEY,
  ALL_MESSAGES_KEY,
  MESSAGE_KEY,
  SEARCH_KEY,
  INSIGHTS_KEY,
  useMailboxes,
  useMessages,
  useInfiniteMessages,
  useAllMessages,
  useMessage,
  useSearch,
  useInfiniteSearch,
  useInsights,
} from './model/useMailbox'
