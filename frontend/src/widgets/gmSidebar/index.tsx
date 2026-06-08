'use client'

import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import {
  LayoutDashboard,
  Mail,
  Archive,
  Trash2,
  Settings,
  LogOut,
} from 'lucide-react'
import GmHttpService from '@/shared/api/gmHttpService'

const navItems = [
  { href: '/gm', label: 'Home', icon: LayoutDashboard, exact: true },
  { href: '/gm/accounts', label: 'Accounts', icon: Mail },
  { href: '/gm/archive', label: 'Archive', icon: Archive },
  { href: '/gm/cleanup', label: 'Cleanup', icon: Trash2 },
]

const api = new GmHttpService()

export function GmSidebar() {
  const pathname = usePathname()
  const router = useRouter()

  const handleLogout = async () => {
    try {
      await api.post('/api/v1/auth/logout')
    } catch {
      /* session may already be gone */
    }
    api.clearToken()
    router.push('/login')
    router.refresh()
  }

  const navLink = (href: string, label: string, Icon: typeof LayoutDashboard, exact?: boolean) => {
    const active = exact ? pathname === href : pathname.startsWith(href)
    return (
      <Link
        key={href}
        href={href}
        className={`group relative flex items-center gap-3 rounded-xl px-3.5 py-2.5 text-[13px] font-medium transition-all duration-300 ${
          active ? 'text-primary-foreground' : 'text-muted-foreground hover:text-foreground'
        }`}
      >
        {active && (
          <div className="absolute inset-0 overflow-hidden rounded-xl">
            <div className="absolute inset-0 bg-gradient-to-r from-primary to-primary/80" />
            <div className="absolute inset-0 bg-gradient-to-r from-transparent to-white/10" />
          </div>
        )}
        {!active && (
          <div className="absolute inset-0 rounded-xl bg-white/[0.03] opacity-0 transition-opacity group-hover:opacity-100" />
        )}
        <Icon size={17} className="relative z-10" strokeWidth={active ? 2.25 : 1.75} />
        <span className="relative z-10">{label}</span>
        {active && (
          <div className="absolute -right-2 top-1/2 h-10 w-10 -translate-y-1/2 rounded-full bg-primary/20 blur-xl" />
        )}
      </Link>
    )
  }

  return (
    <aside className="flex h-screen w-[248px] shrink-0 flex-col border-r border-border/80 bg-sidebar/90 backdrop-blur-xl">
      <div className="px-5 pt-6 pb-5">
        <div className="flex items-center gap-3">
          <div className="relative flex h-9 w-9 items-center justify-center rounded-xl bg-primary/15">
            <div className="absolute inset-0 rounded-xl bg-primary/20 blur-lg" />
            <span className="relative text-lg font-bold text-primary">G</span>
          </div>
          <div>
            <p className="text-[15px] font-bold tracking-tight text-foreground leading-none">
              Giraffe<span className="text-gold">Mail</span>
            </p>
            <p className="mt-0.5 text-[10px] font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Archive
            </p>
          </div>
        </div>
      </div>

      <div className="mx-4 h-px bg-gradient-to-r from-transparent via-border to-transparent" />

      <nav className="flex-1 space-y-0.5 px-3 py-4">
        {navItems.map(({ href, label, icon, exact }) => navLink(href, label, icon, exact))}
      </nav>

      <div className="mx-4 h-px bg-gradient-to-r from-transparent via-border to-transparent" />

      <div className="space-y-0.5 px-3 py-4">
        {navLink('/gm/settings', 'Settings', Settings)}
        <button
          onClick={handleLogout}
          className="group relative flex w-full items-center gap-3 rounded-xl px-3.5 py-2.5 text-[13px] font-medium text-muted-foreground transition-all hover:text-foreground"
        >
          <div className="absolute inset-0 rounded-xl bg-white/[0.03] opacity-0 transition-opacity group-hover:opacity-100" />
          <LogOut size={17} className="relative z-10" strokeWidth={1.75} />
          <span className="relative z-10">Sign out</span>
        </button>
      </div>

      <div className="px-5 pb-5">
        <div className="flex items-center gap-2.5">
          <div className="relative">
            <div className="h-2 w-2 rounded-full bg-emerald-400" />
            <div className="absolute inset-0 h-2 w-2 animate-ping rounded-full bg-emerald-400 opacity-60" />
          </div>
          <span className="text-[11px] font-medium text-muted-foreground">Archive engine ready</span>
        </div>
      </div>
    </aside>
  )
}
