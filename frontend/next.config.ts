import type { NextConfig } from 'next'

const apiProxy = process.env.GM_API_PROXY ?? 'http://localhost:9191'

const nextConfig: NextConfig = {
  ...(process.env.NEXT_OUTPUT === 'export' ? { output: 'export' } : {}),
  images: {
    unoptimized: true,
  },
  async rewrites() {
    if (process.env.NEXT_OUTPUT === 'export') return []
    return [{ source: '/api/:path*', destination: `${apiProxy}/api/:path*` }]
  },
}

export default nextConfig
