'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Loader2 } from 'lucide-react'
import toast from 'react-hot-toast'
import GmHttpService from '@/shared/api/gmHttpService'

const api = new GmHttpService()

export function GmChangePassword() {
  const router = useRouter()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (newPassword.length < 8) {
      setError('New password must be at least 8 characters')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('New passwords do not match')
      return
    }
    setLoading(true)
    try {
      await api.post<{ message: string }>('/api/v1/auth/change_password', {
        current_password: currentPassword,
        new_password: newPassword,
      })
      api.clearToken()
      toast.success('Password changed — sign in again')
      router.push('/login')
      router.refresh()
    } catch (err: unknown) {
      setError((err as Error).message ?? 'Failed to change password')
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="gm-card space-y-3">
      <h2 className="text-[15px] font-semibold text-foreground">Change password</h2>
      {error && (
        <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-[13px] text-red-400">
          {error}
        </div>
      )}
      <div className="space-y-1">
        <label className="text-[12px] text-muted-foreground">Current password</label>
        <input
          type="password"
          className="gm-input"
          value={currentPassword}
          onChange={e => setCurrentPassword(e.target.value)}
          required
          autoComplete="current-password"
        />
      </div>
      <div className="space-y-1">
        <label className="text-[12px] text-muted-foreground">New password</label>
        <input
          type="password"
          className="gm-input"
          value={newPassword}
          onChange={e => setNewPassword(e.target.value)}
          required
          minLength={8}
          autoComplete="new-password"
        />
      </div>
      <div className="space-y-1">
        <label className="text-[12px] text-muted-foreground">Confirm new password</label>
        <input
          type="password"
          className="gm-input"
          value={confirmPassword}
          onChange={e => setConfirmPassword(e.target.value)}
          required
          minLength={8}
          autoComplete="new-password"
        />
      </div>
      <button type="submit" disabled={loading} className="gm-btn-primary">
        {loading && <Loader2 size={13} className="animate-spin" />}
        Update password
      </button>
    </form>
  )
}
