'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'

export default function RootPage() {
  const router = useRouter()
  useEffect(() => {
    router.replace('/login')
  }, [router])
  return (
    <div className="flex min-h-screen items-center justify-center bg-[#0E0E10] text-[#C9A227]">
      Loading…
    </div>
  )
}
