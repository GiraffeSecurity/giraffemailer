import { NextRequest, NextResponse } from 'next/server'

export function middleware(req: NextRequest) {
  const { pathname } = req.nextUrl

  if (pathname === '/gm/search' || pathname.startsWith('/gm/search/')) {
    const archiveUrl = req.nextUrl.clone()
    archiveUrl.pathname = '/gm/archive'
    return NextResponse.redirect(archiveUrl)
  }

  return NextResponse.next()
}

export const config = {
  matcher: ['/gm/:path*'],
}
