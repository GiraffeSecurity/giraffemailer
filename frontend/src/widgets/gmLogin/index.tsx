'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Loader2 } from 'lucide-react'
import GmHttpService from '@/shared/api/gmHttpService'

const api = new GmHttpService()

export function GmLogin() {
  const router = useRouter()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const result = await api.post<{ expires_at: string; message: string; token?: string }>(
        '/api/v1/auth/login',
        { email, password },
      )
      if (result.token) {
        api.setToken(result.token)
      }
      router.push('/gm')
      router.refresh()
    } catch (err: unknown) {
      setError((err as Error).message ?? 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background">
      <div className="gradient-mesh" />
      <div className="noise-overlay" />

      <div className="relative z-10 w-full max-w-sm px-4">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-primary/15">
            <div className="absolute h-12 w-12 rounded-2xl bg-primary/20 blur-xl" />
            <span className="relative text-2xl font-bold text-primary">G</span>
          </div>
          <p className="text-xl font-bold tracking-tight text-foreground">
            Giraffe<span className="text-gold">Mail</span>
          </p>
          <p className="mt-1 text-[13px] text-muted-foreground">Email backup &amp; archive</p>
        </div>

        <form onSubmit={handleSubmit} className="gm-card space-y-4">
          <h1 className="text-[15px] font-semibold text-foreground">Sign in</h1>

          {error && (
            <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-[13px] text-red-400">
              {error}
            </div>
          )}

          <div className="space-y-1">
            <label className="text-[12px] text-muted-foreground">Email</label>
            <input
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              placeholder="admin@localhost"
              required
              autoFocus
              className="gm-input"
            />
          </div>

          <div className="space-y-1">
            <label className="text-[12px] text-muted-foreground">Password</label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              placeholder="••••••••"
              required
              className="gm-input"
            />
          </div>

          <button type="submit" disabled={loading} className="gm-btn-primary w-full">
            {loading && <Loader2 size={14} className="animate-spin" />}
            Sign in
          </button>
        </form>
      </div>
    </div>
  )
}
