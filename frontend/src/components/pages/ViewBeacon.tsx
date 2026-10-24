'use client'

import { useEffect, useRef } from 'react'
import { api } from '@/lib/api'

/** Records one view of the public page per mount; renders nothing and never throws. */
export function ViewBeacon({ slug }: { slug: string }) {
  const sent = useRef(false)

  useEffect(() => {
    if (sent.current) return
    sent.current = true
    // The beacon request's own Referer is this page, so the visitor's referrer goes in the body.
    let referrer = ''
    try {
      referrer = document.referrer
    } catch {
      // keep ''
    }
    try {
      const url = `/api/public/pages/${encodeURIComponent(slug)}/view`
      const body = JSON.stringify({ referrer })
      if (typeof navigator !== 'undefined' && typeof navigator.sendBeacon === 'function' && navigator.sendBeacon(url, body)) {
        return
      }
    } catch {
      // fall through to fetch
    }
    try {
      Promise.resolve(api.recordPageView(slug, referrer)).catch(() => {})
    } catch {
      // view tracking is best-effort
    }
  }, [slug])

  return null
}
