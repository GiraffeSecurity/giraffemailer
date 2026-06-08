'use client'

import { useEffect, useState } from 'react'
import toast from 'react-hot-toast'
import { Play, Trash2, ChevronDown, Loader2, Pencil } from 'lucide-react'
import {
  useCleanupJobs,
  useCleanupRuns,
  useCreateCleanupJob,
  useUpdateCleanupJob,
  useDeleteCleanupJob,
  useRunCleanupJob,
  parseCleanupFilter,
  formatFilterSummary,
  hasOptionalFilters,
  emptyOptionalFilters,
  normalizeCleanupFilter,
  parseOptionalInt,
  cleanupService,
} from '@/entities/cleanup'
import { useMailAccounts } from '@/entities/mailAccount'
import type { CleanupFilter, CleanupJob, CreateJobRequest } from '@/entities/cleanup'

function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    running: 'bg-blue-500/15 text-blue-400',
    done: 'bg-emerald-500/15 text-emerald-400',
    failed: 'bg-red-500/15 text-red-400',
    pending: 'bg-amber-500/15 text-amber-400',
    cancelled: 'bg-secondary text-muted-foreground',
  }
  return (
    <span className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${colors[status] ?? colors.pending}`}>
      {status}
    </span>
  )
}

function RunsPanel({ jobId }: { jobId: string }) {
  const { data, isLoading } = useCleanupRuns(jobId)
  if (isLoading) return <p className="px-4 py-2 text-xs text-muted-foreground">Loading runs…</p>
  if (!data?.length) return <p className="px-4 py-2 text-xs text-muted-foreground">No runs yet</p>
  return (
    <div className="divide-y divide-border/40">
      {data.map(run => (
        <div key={run.id} className="space-y-1 px-4 py-2.5">
          <div className="flex flex-wrap items-center gap-3 text-[12px]">
            <StatusBadge status={run.status} />
            <span className="text-muted-foreground">{run.processed}/{run.total_candidates} processed</span>
            <span className="text-muted-foreground/70">skipped {run.skipped_unarchived} unarchived</span>
            <span className="ml-auto font-semibold text-primary">{formatBytes(run.freed_bytes)} freed</span>
            {run.error_message && (
              <span className="max-w-48 truncate text-red-400" title={run.error_message}>
                {run.error_message}
              </span>
            )}
          </div>
          {run.status === 'done' && run.total_candidates === 0 && (
            <p className="text-[11px] text-amber-400/90">
              No messages matched this filter. Edit the job, use Preview, or choose “All archived mail”.
            </p>
          )}
        </div>
      ))}
    </div>
  )
}

function JobForm({
  job,
  onClose,
}: {
  job?: CleanupJob
  onClose: () => void
}) {
  const isEdit = !!job
  const { data: accounts } = useMailAccounts()
  const create = useCreateCleanupJob()
  const update = useUpdateCleanupJob()

  const initialFilter = job ? parseCleanupFilter(job) : { account_id: '' }

  const [action, setAction] = useState<'delete' | 'move'>(job?.action ?? 'delete')
  const [name, setName] = useState(job?.name ?? '')
  const [moveTarget, setMoveTarget] = useState(job?.move_target_folder ?? '')
  const [filter, setFilter] = useState<CleanupFilter>(initialFilter)
  const [allArchived, setAllArchived] = useState(!hasOptionalFilters(initialFilter))
  const [preview, setPreview] = useState<{ count: number; total_bytes: number } | null>(null)
  const [previewing, setPreviewing] = useState(false)

  useEffect(() => {
    if (allArchived) {
      setFilter(f => emptyOptionalFilters(f))
      setPreview(null)
    }
  }, [allArchived])

  const activeFilter = normalizeCleanupFilter(allArchived ? emptyOptionalFilters(filter) : filter)

  const handlePreview = async () => {
    if (!activeFilter.account_id) { toast.error('Select an account'); return }
    setPreviewing(true)
    try {
      const p = await cleanupService.preview(activeFilter)
      setPreview(p)
    } catch (e: unknown) {
      toast.error((e as Error).message)
    } finally {
      setPreviewing(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name || !activeFilter.account_id) { toast.error('Name and account required'); return }
    const req: CreateJobRequest = { name, filter: activeFilter, action }
    if (action === 'move') req.move_target_folder = moveTarget
    try {
      if (isEdit) {
        await update.mutateAsync({ id: job!.id, data: req })
        toast.success('Job updated')
      } else {
        await create.mutateAsync(req)
        toast.success('Cleanup job created')
      }
      onClose()
    } catch (e: unknown) {
      toast.error((e as Error).message)
    }
  }

  const pending = create.isPending || update.isPending

  return (
    <form onSubmit={handleSubmit} className="gm-card space-y-3">
      <h3 className="text-[14px] font-semibold text-foreground">
        {isEdit ? 'Edit cleanup job' : 'New cleanup job'}
      </h3>

      <input className="gm-input" placeholder="Job name" value={name} onChange={e => setName(e.target.value)} required />

      <select
        className="gm-input"
        value={activeFilter.account_id}
        onChange={e => setFilter(f => ({ ...f, account_id: e.target.value }))}
        required
      >
        <option value="">Select account…</option>
        {accounts?.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}
      </select>

      <label className="flex items-start gap-2 rounded-lg border border-border/60 bg-secondary/30 px-3 py-2.5 text-[13px]">
        <input
          type="checkbox"
          checked={allArchived}
          onChange={e => setAllArchived(e.target.checked)}
          className="mt-0.5 accent-primary"
        />
        <span>
          <span className="font-medium text-foreground">All archived mail</span>
          <span className="mt-0.5 block text-[12px] text-muted-foreground">
            Remove every message that is already backed up locally. Unarchived mail is always skipped.
          </span>
        </span>
      </label>

      {!allArchived && (
        <>
        <div className="grid grid-cols-2 gap-2">
          <input
            className="gm-input"
            placeholder="Mailbox (e.g. INBOX)"
            value={filter.mailbox_name ?? ''}
            onChange={e => setFilter(f => ({ ...f, mailbox_name: e.target.value || undefined }))}
          />
          <input
            className="gm-input"
            placeholder="Sender domain (e.g. spam.com)"
            value={filter.sender_domain ?? ''}
            onChange={e => setFilter(f => ({ ...f, sender_domain: e.target.value || undefined }))}
          />
          <input
            className="gm-input"
            placeholder="Sender email"
            value={filter.sender_email ?? ''}
            onChange={e => setFilter(f => ({ ...f, sender_email: e.target.value || undefined }))}
          />
          <input
            className="gm-input"
            placeholder="Subject contains"
            value={filter.subject_contains ?? ''}
            onChange={e => setFilter(f => ({ ...f, subject_contains: e.target.value || undefined }))}
          />
          <input
            className="gm-input"
            type="number"
            min={0}
            placeholder="Min age in days (empty = today OK)"
            value={filter.older_than_days ?? ''}
            onChange={e => setFilter(f => ({
              ...f,
              older_than_days: parseOptionalInt(e.target.value),
            }))}
          />
          <input
            className="gm-input"
            type="number"
            min={0}
            placeholder="Min size in KB (empty = any size)"
            value={filter.larger_than_kb ?? ''}
            onChange={e => setFilter(f => ({
              ...f,
              larger_than_kb: parseOptionalInt(e.target.value),
            }))}
          />
        </div>
        <p className="text-[11px] text-muted-foreground">
          Leave age and size empty to include today&apos;s mail and small messages. &quot;1 day&quot; means older than 24 hours — not today.
        </p>
        </>
      )}

      <div className="flex flex-wrap gap-4">
        <label className="flex items-center gap-1.5 text-[13px] text-muted-foreground">
          <input type="radio" value="delete" checked={action === 'delete'} onChange={() => setAction('delete')} className="accent-primary" />
          Delete from server
        </label>
        <label className="flex items-center gap-1.5 text-[13px] text-muted-foreground">
          <input type="radio" value="move" checked={action === 'move'} onChange={() => setAction('move')} className="accent-primary" />
          Move to folder
        </label>
      </div>
      {action === 'move' && (
        <input className="gm-input" placeholder="Target folder name" value={moveTarget} onChange={e => setMoveTarget(e.target.value)} required />
      )}

      {preview && (
        <div className="rounded-lg border border-primary/25 bg-primary/10 p-3 text-[13px] text-primary">
          Preview: {preview.count.toLocaleString()} messages · {formatBytes(preview.total_bytes)}
        </div>
      )}

      <div className="flex flex-wrap gap-2">
        <button type="button" onClick={handlePreview} disabled={previewing} className="gm-btn-ghost !text-[12px]">
          {previewing && <Loader2 size={11} className="animate-spin" />}
          Preview
        </button>
        <button type="submit" disabled={pending} className="gm-btn-primary">
          {pending && <Loader2 size={11} className="animate-spin" />}
          {isEdit ? 'Save changes' : 'Create job'}
        </button>
        <button type="button" onClick={onClose} className="gm-btn-ghost">Cancel</button>
      </div>
    </form>
  )
}

export function GmCleanup() {
  const { data: jobs, isLoading } = useCleanupJobs()
  const { data: accounts } = useMailAccounts()
  const deleteJob = useDeleteCleanupJob()
  const runJob = useRunCleanupJob()
  const [formMode, setFormMode] = useState<'closed' | 'create' | 'edit'>('closed')
  const [editingJob, setEditingJob] = useState<CleanupJob | null>(null)
  const [expanded, setExpanded] = useState<string | null>(null)

  const accountName = (id: string) => accounts?.find(a => a.id === id)?.name ?? 'Unknown account'

  const handleRun = async (id: string) => {
    try {
      await runJob.mutateAsync(id)
      toast.success('Job started')
      setExpanded(id)
    } catch (e: unknown) {
      toast.error((e as Error).message)
    }
  }

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete job "${name}"?`)) return
    try {
      await deleteJob.mutateAsync(id)
      toast.success('Job deleted')
    } catch {
      toast.error('Delete failed')
    }
  }

  const openCreate = () => {
    setEditingJob(null)
    setFormMode('create')
  }

  const openEdit = (job: CleanupJob) => {
    setEditingJob(job)
    setFormMode('edit')
  }

  const closeForm = () => {
    setFormMode('closed')
    setEditingJob(null)
  }

  return (
    <div className="h-full overflow-auto p-8 lg:p-10 animate-slide-up">
      <div className="mx-auto max-w-3xl space-y-5">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h1 className="text-xl font-bold tracking-tight text-foreground">Cleanup</h1>
            <p className="mt-1 max-w-lg text-[13px] text-muted-foreground">
              Remove or move mail on the IMAP server after it is fully archived locally.
              Mail that has not been synced yet is always skipped — never deleted.
            </p>
          </div>
          <button onClick={openCreate} className="gm-btn-primary shrink-0">
            New job
          </button>
        </div>

        {formMode === 'create' && <JobForm onClose={closeForm} />}
        {formMode === 'edit' && editingJob && <JobForm job={editingJob} onClose={closeForm} />}

        {isLoading && <p className="text-[13px] text-muted-foreground">Loading…</p>}

        <div className="space-y-2">
          {jobs?.map(job => {
            const filter = parseCleanupFilter(job)
            return (
              <div key={job.id} className="gm-card !p-0 overflow-hidden">
                <div className="flex items-start gap-3 px-4 py-3.5">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-[14px] font-semibold text-foreground">{job.name}</span>
                      <span className="rounded bg-secondary px-1.5 py-0.5 text-[10px] font-medium uppercase text-muted-foreground">
                        {job.action}
                      </span>
                    </div>
                    <p className="mt-1 text-[12px] text-muted-foreground">
                      {accountName(job.account_id)} · {formatFilterSummary(filter)}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-1">
                    <button
                      onClick={() => openEdit(job)}
                      className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"
                      title="Edit job"
                    >
                      <Pencil size={13} />
                    </button>
                    <button
                      onClick={() => handleRun(job.id)}
                      disabled={runJob.isPending}
                      className="gm-btn-soft !py-1 !px-2.5 !text-[12px] !text-emerald-400 !border-emerald-500/30"
                    >
                      <Play size={10} />
                      Run
                    </button>
                    <button
                      onClick={() => setExpanded(expanded === job.id ? null : job.id)}
                      className="rounded-md p-1.5 text-muted-foreground hover:text-foreground"
                    >
                      <ChevronDown size={15} className={`transition-transform ${expanded === job.id ? 'rotate-180' : ''}`} />
                    </button>
                    <button
                      onClick={() => handleDelete(job.id, job.name)}
                      className="rounded-md p-1.5 text-muted-foreground hover:text-red-400"
                      title="Delete job"
                    >
                      <Trash2 size={13} />
                    </button>
                  </div>
                </div>
                {expanded === job.id && (
                  <div className="border-t border-border/60">
                    <RunsPanel jobId={job.id} />
                  </div>
                )}
              </div>
            )
          })}
          {!isLoading && (!jobs || jobs.length === 0) && formMode === 'closed' && (
            <div className="rounded-xl border border-dashed border-border p-16 text-center">
              <Trash2 size={24} className="mx-auto mb-2 text-muted-foreground/40" />
              <p className="text-[13px] text-muted-foreground">No cleanup jobs yet.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
