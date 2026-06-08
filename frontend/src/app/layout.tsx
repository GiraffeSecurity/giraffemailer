import type { Metadata } from 'next'
import { Inter, Noto_Sans_Georgian } from 'next/font/google'
import './globals.css'
import { Providers } from './providers'

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-inter',
})

const notoGeorgian = Noto_Sans_Georgian({
  subsets: ['georgian'],
  variable: '--font-georgian',
  weight: ['400', '500', '600'],
})

export const metadata: Metadata = {
  title: 'GiraffeMail Archive',
  description: 'Self-hosted email backup and cleanup',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${inter.variable} ${notoGeorgian.variable} dark`}>
      <body className="font-sans antialiased">
        <Providers>{children}</Providers>
      </body>
    </html>
  )
}
