// Empty = same-origin (embedded UI or Next.js rewrite proxy).
// Set NEXT_PUBLIC_GM_API_URL only when the UI must call a different API origin.
export const GM_API_URL = process.env.NEXT_PUBLIC_GM_API_URL ?? ''
