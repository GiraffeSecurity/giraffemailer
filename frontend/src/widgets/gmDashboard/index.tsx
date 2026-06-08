'use client'

import Link from 'next/link'
import { Archive, Mail, Trash2, ArrowUpRight } from 'lucide-react'
import { useInsights } from '@/entities/mailbox'
import { useMailAccounts } from '@/entities/mailAccount'

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

const quickLinks = [
  { href: '/gm/accounts', label: 'Add mail account', icon: Mail, desc: 'Connect IMAP and start archiving' },
  { href: '/gm/archive', label: 'Browse archive', icon: Archive, desc: 'Search and read backed-up messages' },
  { href: '/gm/cleanup', label: 'Free server space', icon: Trash2, desc: 'Delete archived mail from the server' },
]

export function GmDashboard() {
  const { data: insights, isLoading } = useInsights()
  const { data: accounts } = useMailAccounts()

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-border border-t-primary" />
      </div>
    )
  }

  const archivedPct = insights
    ? ((insights.archived_messages / Math.max(insights.total_messages, 1)) * 100).toFixed(0)
    : '0'

  return (
    <div className="h-full overflow-auto p-8 lg:p-10 animate-slide-up">
      <div className="mx-auto max-w-5xl space-y-8">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">Dashboard</h1>
          <p className="mt-1.5 max-w-xl text-[14px] text-muted-foreground">
            Backup first, cleanup second — every message archived locally before it can be removed from the server.
          </p>
        </div>

        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {[
            { label: 'Messages', value: insights?.total_messages.toLocaleString() ?? '—', sub: `${archivedPct}% archived` },
            { label: 'Archived', value: insights?.archived_messages.toLocaleString() ?? '—', sub: formatBytes(insights?.archived_bytes ?? 0) },
            { label: 'Reclaimable', value: formatBytes(insights?.reclaimable_bytes ?? 0), sub: 'safe to remove' },
            { label: 'Accounts', value: String(accounts?.length ?? 0), sub: 'IMAP connections' },
          ].map((s) => (
            <div key={s.label} className="gm-card">
              <p className="text-[12px] text-muted-foreground">{s.label}</p>
              <p className="gm-stat-value mt-1">{s.value}</p>
              {s.sub && <p className="mt-0.5 text-[11px] text-muted-foreground/80">{s.sub}</p>}
            </div>
          ))}
        </div>

        <div className="grid gap-3 sm:grid-cols-3">
          {quickLinks.map(({ href, label, icon: Icon, desc }) => (
            <Link key={href} href={href} className="gm-card-hover group flex items-start gap-3">
              <div className="rounded-lg bg-primary/15 p-2.5">
                <Icon size={16} className="text-primary" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-[13px] font-semibold text-foreground">{label}</p>
                <p className="mt-0.5 text-[12px] text-muted-foreground">{desc}</p>
              </div>
              <ArrowUpRight size={14} className="mt-1 shrink-0 text-muted-foreground/40 transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-primary" />
            </Link>
          ))}
        </div>

        {insights?.top_senders && insights.top_senders.length > 0 && (
          <div className="gm-card !p-0">
            <div className="border-b border-border px-5 py-3.5">
              <h2 className="text-[13px] font-semibold text-foreground">Largest senders</h2>
            </div>
            <div className="divide-y divide-border/50">
              {insights.top_senders.slice(0, 8).map((s) => (
                <div key={s.sender_email} className="flex items-center gap-3 px-5 py-2.5 transition-colors hover:bg-white/[0.02]">
                  <span className="flex-1 truncate text-[13px] text-foreground/90">{s.sender_email}</span>
                  <span className="text-[12px] text-muted-foreground">{s.count.toLocaleString()}</span>
                  <span className="w-16 text-right text-[12px] font-semibold text-primary">{formatBytes(s.total_bytes)}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
