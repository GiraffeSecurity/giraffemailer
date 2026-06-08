// Browser: same-origin (embedded UI or Next dev proxy). SSR/build: explicit API host.
export const GM_API_URL =
  typeof window !== 'undefined'
    ? (process.env.NEXT_PUBLIC_GM_API_URL ?? '')
    : (process.env.NEXT_PUBLIC_GM_API_URL ?? 'http://localhost:9191')
