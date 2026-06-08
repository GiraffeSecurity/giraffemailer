'use client'

import { useState, useRef, useCallback, useEffect } from 'react'
import DOMPurify from 'dompurify'
import { Paperclip, Archive, Loader2, Search, Download } from 'lucide-react'
import { useAllMessages, useInfiniteSearch, useMessage } from '@/entities/mailbox'
import { useMailAccounts } from '@/entities/mailAccount'
import type { Message } from '@/entities/mailbox'
import { downloadAttachment, normalizeAttachments } from '@/shared/lib/attachments'

function dedupeMessages(msgs: Message[]): Message[] {
  const seen = new Set<string>()
  return msgs.filter((m) => {
    if (seen.has(m.id)) return false
    seen.add(m.id)
    return true
  })
}

function fmtBytes(b: number | undefined | null) {
  if (!b || b <= 0) return ''
  if (b < 1048576) return `${(b / 1024).toFixed(0)} KB`
  return `${(b / 1048576).toFixed(1)} MB`
}

function fmtDate(d: string | null) {
  if (!d) return ''
  const dt = new Date(d)
  const now = new Date()
  const diff = now.getTime() - dt.getTime()
  if (diff < 86400000) return dt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (diff < 86400000 * 365) return dt.toLocaleDateString([], { month: 'short', day: 'numeric' })
  return dt.toLocaleDateString([], { year: 'numeric', month: 'short', day: 'numeric' })
}

function Avatar({ email }: { email: string }) {
  const letter = (email[0] || '?').toUpperCase()
  const hue = email.split('').reduce((n, c) => n + c.charCodeAt(0), 0) % 360
  return (
    <div
      className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold text-white"
      style={{ backgroundColor: `hsl(${hue},35%,42%)` }}
    >
      {letter}
    </div>
  )
}

function MessageRow({ msg, selected, onSelect }: { msg: Message; selected: boolean; onSelect: () => void }) {
  return (
    <button
      onClick={onSelect}
      className={`flex w-full items-start gap-3 border-b border-border/40 px-3 py-2.5 text-left transition-colors ${
        selected ? 'bg-primary/8' : 'hover:bg-white/[0.03]'
      }`}
    >
      <Avatar email={msg.sender_email} />
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-1">
          <span className={`truncate text-[13px] font-medium ${selected ? 'text-primary' : 'text-foreground'}`}>
            {msg.sender_name || msg.sender_email}
          </span>
          <span className="shrink-0 text-[11px] text-muted-foreground">{fmtDate(msg.date)}</span>
        </div>
        <div className="flex items-center gap-1">
          <span className="truncate text-[12px] text-muted-foreground">{msg.subject ?? '(no subject)'}</span>
          {msg.has_attachments && <Paperclip size={10} className="shrink-0 text-muted-foreground/50" />}
          {msg.archive_state === 'deleted_from_server' && <Archive size={10} className="shrink-0 text-primary" />}
        </div>
        {msg.body_preview && (
          <p className="truncate text-[11px] text-muted-foreground/60">{msg.body_preview}</p>
        )}
      </div>
    </button>
  )
}

function MessagePane({ id }: { id: string }) {
  const { data, isLoading } = useMessage(id)
  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 size={18} className="animate-spin text-muted-foreground" />
      </div>
    )
  }
  if (!data) return null

  const safeHtml = data.body_html
    ? DOMPurify.sanitize(data.body_html, { USE_PROFILES: { html: true } })
    : null
  const attachments = normalizeAttachments(data.attachments)

  return (
    <div className="flex h-full flex-col overflow-auto bg-card/50">
      <div className="border-b border-border px-6 py-4">
        <h2 className="text-base font-semibold text-foreground">{data.subject ?? '(no subject)'}</h2>
        <div className="mt-2 flex items-center gap-2.5">
          <Avatar email={data.sender_email ?? ''} />
          <div>
            <p className="text-[13px] font-medium text-foreground">
              {data.sender_name || data.sender_email}
            </p>
            <p className="text-[11px] text-muted-foreground">
              {data.sender_name && data.sender_email && `<${data.sender_email}>`}
              {data.date && ` · ${new Date(data.date).toLocaleString()}`}
            </p>
          </div>
        </div>
      </div>
      <div className="flex-1 overflow-auto px-6 py-5">
        {safeHtml ? (
          <div
            className="prose prose-invert prose-sm max-w-none text-foreground/90 [&_a]:text-primary"
            dangerouslySetInnerHTML={{ __html: safeHtml }}
          />
        ) : (
          <pre className="whitespace-pre-wrap font-sans text-[13px] leading-relaxed text-foreground/80">
            {data.body_text}
          </pre>
        )}
      </div>
      {attachments.length > 0 && (
        <div className="border-t border-border px-6 py-3">
          <p className="mb-2 text-[11px] font-medium text-muted-foreground">Attachments</p>
          <div className="flex flex-wrap gap-2">
            {attachments.map((att, index) => (
              <button
                key={`${data.id}-att-${index}-${att.part_path}`}
                onClick={() => downloadAttachment(data.id, att.part_path, att.filename)}
                className="flex items-center gap-1.5 rounded-lg border border-border bg-secondary/40 px-2.5 py-1.5 transition-colors hover:border-primary/30 hover:bg-primary/10"
              >
                <Paperclip size={11} className="text-muted-foreground" />
                <span className="text-[12px] text-foreground">{att.filename}</span>
                {fmtBytes(att.size_bytes) && (
                  <span className="text-[11px] text-muted-foreground">{fmtBytes(att.size_bytes)}</span>
                )}
                <Download size={10} className="text-muted-foreground" />
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

export function GmArchive() {
  const { data: accounts } = useMailAccounts()
  const [accountId, setAccountId] = useState('')
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 250)
    return () => clearTimeout(t)
  }, [query])

  const browseParams = accountId ? { account_id: accountId } : {}
  const searchParams = {
    q: debouncedQuery,
    ...(accountId ? { account_id: accountId } : {}),
  }

  const browse = useAllMessages(debouncedQuery ? undefined : browseParams)
  const search = useInfiniteSearch(searchParams, !!debouncedQuery)

  const browseMsgs = dedupeMessages(browse.data?.pages.flatMap((p) => p.messages) ?? [])
  const searchMsgs = dedupeMessages(search.data?.pages.flatMap((p) => p.messages) ?? [])
  const msgs = debouncedQuery ? searchMsgs : browseMsgs
  const searchTotal = search.data?.pages[0]?.total

  const isLoading = debouncedQuery ? search.isLoading : browse.isLoading
  const isFetchingNext = debouncedQuery ? search.isFetchingNextPage : browse.isFetchingNextPage
  const hasMore = debouncedQuery ? search.hasNextPage : browse.hasNextPage

  const observerRef = useRef<IntersectionObserver | null>(null)
  const loadMoreRef = useCallback(
    (node: HTMLDivElement | null) => {
      if (observerRef.current) observerRef.current.disconnect()
      if (!node) return
      observerRef.current = new IntersectionObserver((entries) => {
        if (!entries[0].isIntersecting || isFetchingNext || !hasMore) return
        if (debouncedQuery) search.fetchNextPage()
        else browse.fetchNextPage()
      })
      observerRef.current.observe(node)
    },
    [hasMore, isFetchingNext, debouncedQuery, browse, search],
  )

  return (
    <div className="flex h-full flex-col">
      <div className="flex shrink-0 items-center gap-3 border-b border-border/80 bg-card/40 px-4 py-3 backdrop-blur-sm">
        <div className="relative flex flex-1 items-center">
          <Search size={14} className="absolute left-3 text-muted-foreground" />
          <input
            value={query}
            onChange={(e) => { setQuery(e.target.value); setSelectedId(null) }}
            placeholder="Search archived messages…"
            className="gm-input pl-9"
          />
        </div>
        <select
          value={accountId}
          onChange={(e) => { setAccountId(e.target.value); setSelectedId(null) }}
          className="gm-input w-auto min-w-[140px]"
        >
          <option value="">All accounts</option>
          {accounts?.map((a) => (
            <option key={a.id} value={a.id}>{a.name}</option>
          ))}
        </select>
        {debouncedQuery && searchTotal != null && (
          <span className="shrink-0 text-[12px] text-muted-foreground">
            {searchTotal.toLocaleString()} results
          </span>
        )}
      </div>

      <div className="flex flex-1 overflow-hidden">
        <div className="flex w-[300px] shrink-0 flex-col overflow-y-auto border-r border-border/80 bg-background/50">
          {isLoading && !msgs.length && (
            <div className="flex items-center justify-center py-16">
              <Loader2 size={16} className="animate-spin text-muted-foreground" />
            </div>
          )}
          {!isLoading && msgs.length === 0 && (
            <p className="py-16 text-center text-[13px] text-muted-foreground">
              {debouncedQuery ? 'No matching messages' : 'No messages yet'}
            </p>
          )}
          {msgs.map((msg) => (
            <MessageRow
              key={msg.id}
              msg={msg}
              selected={selectedId === msg.id}
              onSelect={() => setSelectedId(msg.id)}
            />
          ))}
          <div ref={loadMoreRef} className="py-3 text-center">
            {isFetchingNext && <Loader2 size={14} className="mx-auto animate-spin text-muted-foreground" />}
          </div>
        </div>

        <div className="flex flex-1 overflow-hidden">
          {selectedId ? (
            <MessagePane id={selectedId} />
          ) : (
            <div className="flex h-full w-full flex-col items-center justify-center gap-2 text-muted-foreground/40">
              <Archive size={28} strokeWidth={1.25} />
              <p className="text-[13px]">Select a message to read</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
