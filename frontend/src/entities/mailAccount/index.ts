export type { MailAccount, CreateMailAccountRequest } from './model/types'
export { mailAccountService } from './api/mailAccountService'
export {
  MAIL_ACCOUNTS_KEY,
  useMailAccounts,
  useMailAccount,
  useCreateMailAccount,
  useDeleteMailAccount,
  useSyncMailAccount,
} from './model/useMailAccounts'
