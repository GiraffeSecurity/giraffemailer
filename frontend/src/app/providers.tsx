'use client'

import { QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'react-hot-toast'
import { queryClient } from '@/shared/lib/queryClient'

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      {children}
      <Toaster
        position="top-right"
        toastOptions={{
          duration: 4000,
          style: {
            background: '#181B1F',
            color: '#F0F2F5',
            border: '1px solid #252830',
            borderRadius: '0.75rem',
            fontSize: '0.875rem',
            boxShadow: '0 8px 32px rgba(0,0,0,0.5)',
          },
          success: { iconTheme: { primary: '#C9A227', secondary: '#181B1F' } },
        }}
      />
    </QueryClientProvider>
  )
}
