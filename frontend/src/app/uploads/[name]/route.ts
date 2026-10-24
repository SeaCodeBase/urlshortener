import { NextRequest } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:8081'

/** Upload names the backend generates: 32 hex chars + extension. */
const UPLOAD_NAME = /^[0-9a-f]{32}\.(png|jpg)$/

const PASSTHROUGH_HEADERS = ['Content-Type', 'Cache-Control'] as const

/** Serves an uploaded image (QR logo) from the backend's /uploads/{name}. */
export async function GET(_request: NextRequest, { params }: { params: Promise<{ name: string }> }) {
  const { name } = await params
  if (!UPLOAD_NAME.test(name)) {
    return new Response('Not found', { status: 404 })
  }

  const upstream = await fetch(`${BACKEND_URL}/uploads/${name}`)
  const headers = new Headers()
  for (const header of PASSTHROUGH_HEADERS) {
    const value = upstream.headers.get(header)
    if (value != null) headers.set(header, value)
  }
  return new Response(upstream.body, { status: upstream.status, headers })
}
