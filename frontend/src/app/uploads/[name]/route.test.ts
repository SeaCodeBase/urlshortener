import { afterEach, describe, expect, it, vi } from 'vitest'
import type { NextRequest } from 'next/server'
import { GET } from './route'

const NAME = `${'a'.repeat(32)}.png`

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('GET /uploads/[name]', () => {
  it('passes the image through with nosniff', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response('img', { status: 200, headers: { 'Content-Type': 'image/png' } })),
    )
    const res = await GET({} as NextRequest, { params: Promise.resolve({ name: NAME }) })
    expect(res.status).toBe(200)
    expect(res.headers.get('Content-Type')).toBe('image/png')
    expect(res.headers.get('X-Content-Type-Options')).toBe('nosniff')
  })

  it('404s names the backend never generates', async () => {
    const res = await GET({} as NextRequest, { params: Promise.resolve({ name: '../etc/passwd' }) })
    expect(res.status).toBe(404)
  })
})
