'use client'

import { useState } from 'react'
import toast from 'react-hot-toast'
import { Inbox, ChevronRight, Loader2 } from 'lucide-react'
import {
  useMailAccounts,
  useCreateMailAccount,
  useDeleteMailAccount,
  useSyncMailAccount,
} from '@/entities/mailAccount'
import { useMailboxes } from '@/entities/mailbox'
import type { CreateMailAccountRequest } from '@/entities/mailAccount'
import { mailAccountService } from '@/entities/mailAccount'

function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

function ProgressBar({ pct }: { pct: number }) {
  return (
    <div className="h-1 w-full overflow-hidden rounded-full bg-border">
      <div
        className="h-full rounded-full bg-gradient-to-r from-primary to-primary/70 transition-all"
        style={{ width: `${Math.min(100, pct)}%` }}
      />
    </div>
  )
}

function AccountMailboxes({ accountId }: { accountId: string }) {
  const { data, isLoading } = useMailboxes(accountId)
  if (isLoading) return <p className="px-4 py-3 text-xs text-muted-foreground">Loading mailboxes…</p>
  if (!data?.length) return <p className="px-4 py-3 text-xs text-muted-foreground">No mailboxes indexed</p>
  return (
    <div className="divide-y divide-border/40">
      {data.map(mb => (
        <div key={mb.id} className="px-4 py-2.5">
          <div className="flex items-center justify-between">
            <span className="text-[13px] text-foreground">{mb.name}</span>
            <span className="text-[12px] text-muted-foreground">
              {mb.message_count.toLocaleString()} · {formatBytes(mb.total_size_bytes)}
            </span>
          </div>
          <ProgressBar pct={mb.archived_pct} />
          <p className="mt-0.5 text-[11px] text-muted-foreground">
            {mb.archived_count.toLocaleString()} archived ({mb.archived_pct.toFixed(0)}%)
          </p>
        </div>
      ))}
    </div>
  )
}

function AddAccountForm({ onClose }: { onClose: () => void }) {
  const create = useCreateMailAccount()
  const [form, setForm] = useState<CreateMailAccountRequest>({
    name: '',
    email_address: '',
    imap_host: '',
    imap_port: 993,
    use_tls: true,
    username: '',
    password: '',
  })

  const field = (key: keyof CreateMailAccountRequest) => ({
    value: String(form[key]),
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => {
      const val = key === 'imap_port'
        ? parseInt(e.target.value) || 993
        : key === 'use_tls'
        ? e.target.checked
        : e.target.value
      setForm(f => ({ ...f, [key]: val }))
    },
  })

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await create.mutateAsync(form)
      toast.success('Account added')
      onClose()
    } catch (err: unknown) {
      toast.error((err as Error).message ?? 'Failed to add account')
    }
  }

  return (
    <form onSubmit={submit} className="gm-card space-y-3">
      <h3 className="text-[14px] font-semibold text-foreground">Add IMAP account</h3>
      <div className="grid grid-cols-2 gap-2">
        <input className="gm-input" placeholder="Name" {...field('name')} required />
        <input className="gm-input" placeholder="Email address" type="email" {...field('email_address')} required />
        <input className="gm-input" placeholder="IMAP host" {...field('imap_host')} required />
        <input className="gm-input" placeholder="Port (993)" type="number" {...field('imap_port')} required />
        <input className="gm-input" placeholder="Username" {...field('username')} required />
        <input className="gm-input" placeholder="Password" type="password" {...field('password')} required />
      </div>
      <label className="flex items-center gap-2 text-[13px] text-muted-foreground">
        <input
          type="checkbox"
          checked={!!form.use_tls}
          onChange={e => setForm(f => ({ ...f, use_tls: e.target.checked }))}
          className="accent-primary"
        />
        Use TLS/SSL
      </label>
      <div className="flex gap-2">
        <button type="submit" disabled={create.isPending} className="gm-btn-primary">
          {create.isPending && <Loader2 size={13} className="animate-spin" />}
          Add account
        </button>
        <button type="button" onClick={onClose} className="gm-btn-ghost">Cancel</button>
      </div>
    </form>
  )
}

export function GmAccountsList() {
  const { data: accounts, isLoading } = useMailAccounts()
  const deleteAccount = useDeleteMailAccount()
  const syncAccount = useSyncMailAccount()
  const [expanded, setExpanded] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)
  const [testing, setTesting] = useState<string | null>(null)
  const [syncing, setSyncing] = useState<string | null>(null)

  const handleSync = async (id: string) => {
    setSyncing(id)
    try {
      await syncAccount.mutateAsync(id)
      toast.success('Sync started')
    } catch {
      toast.error('Sync failed')
    } finally {
      setSyncing(null)
    }
  }

  const handleTest = async (id: string) => {
    setTesting(id)
    try {
      const r = await mailAccountService.testConnection(id)
      if (r.success) toast.success('Connection OK')
      else toast.error(`Connection failed: ${r.error}`)
    } catch {
      toast.error('Test failed')
    } finally {
      setTesting(null)
    }
  }

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete account "${name}"?`)) return
    try {
      await deleteAccount.mutateAsync(id)
      toast.success('Account deleted')
    } catch {
      toast.error('Delete failed')
    }
  }

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-border border-t-primary" />
      </div>
    )
  }

  return (
    <div className="h-full overflow-auto p-8 lg:p-10 animate-slide-up">
      <div className="mx-auto max-w-3xl space-y-5">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-xl font-bold tracking-tight text-foreground">Accounts</h1>
            <p className="mt-1 text-[13px] text-muted-foreground">Connect IMAP mailboxes for backup</p>
          </div>
          <button onClick={() => setAdding(v => !v)} className="gm-btn-primary">
            Add account
          </button>
        </div>

        {adding && <AddAccountForm onClose={() => setAdding(false)} />}

        {(!accounts || accounts.length === 0) && !adding && (
          <div className="rounded-xl border border-dashed border-border p-16 text-center">
            <Inbox size={24} className="mx-auto mb-2 text-muted-foreground/40" />
            <p className="text-[13px] text-muted-foreground">No accounts yet. Add your first IMAP account.</p>
          </div>
        )}

        <div className="space-y-2">
          {accounts?.map(account => (
            <div key={account.id} className="gm-card !p-0 overflow-hidden">
              <button
                onClick={() => setExpanded(expanded === account.id ? null : account.id)}
                className="flex w-full items-center gap-3 px-4 py-3.5 text-left"
              >
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-[14px] font-semibold text-foreground">{account.name}</span>
                    <span className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${account.sync_enabled ? 'bg-emerald-500/15 text-emerald-400' : 'bg-secondary text-muted-foreground'}`}>
                      {account.sync_enabled ? 'syncing' : 'paused'}
                    </span>
                  </div>
                  <p className="mt-0.5 text-[12px] text-muted-foreground">
                    {account.email_address} · {account.imap_host}:{account.imap_port}
                    {account.last_sync_at && ` · synced ${new Date(account.last_sync_at).toLocaleDateString()}`}
                  </p>
                </div>
                <ChevronRight
                  size={15}
                  className={`text-muted-foreground transition-transform ${expanded === account.id ? 'rotate-90' : ''}`}
                />
              </button>

              {expanded === account.id && (
                <div className="border-t border-border/60">
                  <AccountMailboxes accountId={account.id} />
                  <div className="flex gap-2 border-t border-border/60 px-4 py-3">
                    <button onClick={() => handleSync(account.id)} disabled={syncing === account.id} className="gm-btn-soft !py-1.5 !text-[12px]">
                      {syncing === account.id && <Loader2 size={11} className="animate-spin" />}
                      Sync now
                    </button>
                    <button onClick={() => handleTest(account.id)} disabled={testing === account.id} className="gm-btn-ghost !py-1.5 !text-[12px]">
                      {testing === account.id && <Loader2 size={11} className="animate-spin" />}
                      Test connection
                    </button>
                    <button onClick={() => handleDelete(account.id, account.name)} className="gm-btn-ghost !py-1.5 !text-[12px] !text-red-400 hover:!text-red-300">
                      Delete
                    </button>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
