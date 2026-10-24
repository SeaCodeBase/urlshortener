'use client'

import { useEffect, useRef } from 'react'
import { api } from '@/lib/api'

/** Records one view of the public page per mount; renders nothing and never throws. */
export function ViewBeacon({ slug }: { slug: string }) {
  const sent = useRef(false)

  useEffect(() => {
    if (sent.current) return
    sent.current = true
    try {
      const url = `/api/public/pages/${encodeURIComponent(slug)}/view`
      if (typeof navigator !== 'undefined' && typeof navigator.sendBeacon === 'function' && navigator.sendBeacon(url, '')) {
        return
      }
    } catch {
      // fall through to fetch
    }
    try {
      Promise.resolve(api.recordPageView(slug)).catch(() => {})
    } catch {
      // view tracking is best-effort
    }
  }, [slug])

  return null
}
