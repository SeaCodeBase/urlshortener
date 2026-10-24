'use client'

import { useState } from 'react'
import { Globe } from 'lucide-react'
import { useMeta } from '@/lib/hooks'
import { cn } from '@/lib/utils'

function hostOf(url: string): string | null {
  try {
    return new URL(url).hostname || null
  } catch {
    return null
  }
}

/** Destination favicon via Google's favicon service (or `src`), Globe icon fallback. */
export function Favicon({
  url,
  src,
  size = 32,
  className,
}: {
  url: string
  src?: string
  size?: number
  className?: string
}) {
  const { data: meta } = useMeta()
  const host = hostOf(url)
  const imageSrc = src || (host ? `https://www.google.com/s2/favicons?domain=${encodeURIComponent(host)}&sz=32` : '')
  const [failedSrc, setFailedSrc] = useState<string | null>(null)
  const disabled = meta?.favicon_service === false

  const box = cn(
    'inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full border border-border bg-white',
    className,
  )
  if (disabled || !imageSrc || failedSrc === imageSrc) {
    return (
      <span className={box} style={{ width: size, height: size }} aria-hidden>
        <Globe className="text-muted-foreground" style={{ width: size * 0.55, height: size * 0.55 }} />
      </span>
    )
  }
  return (
    <span className={box} style={{ width: size, height: size }} aria-hidden>
      {/* eslint-disable-next-line @next/next/no-img-element -- external favicon, no optimisation needed */}
      <img
        src={imageSrc}
        alt=""
        width={size * 0.6}
        height={size * 0.6}
        loading="lazy"
        referrerPolicy="no-referrer"
        onError={() => setFailedSrc(imageSrc)}
      />
    </span>
  )
}
