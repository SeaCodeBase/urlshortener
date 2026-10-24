'use client'

import { useEffect, useMemo, useRef } from 'react'
import type QRCodeStyling from 'qr-code-styling'
import { frameInsets, toStylingOptions } from '@/lib/qr/design'
import { cn } from '@/lib/utils'
import type { QrDesign } from '@/types'

interface QrPreviewProps {
  design: QrDesign
  /** The content encoded in the code (the QR URL). */
  data: string
  size?: number
  className?: string
}

/** Live QR code preview with its frame, laid out like the exported image. */
export function QrPreview({ design, data, size = 200, className }: QrPreviewProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const instanceRef = useRef<QRCodeStyling | null>(null)
  const options = useMemo(() => toStylingOptions(design, data, size), [design, data, size])
  const insets = frameInsets(design, size)

  useEffect(() => {
    let cancelled = false
    // qr-code-styling needs the DOM at import time: load it on the client only.
    import('qr-code-styling')
      .then(({ default: Styling }) => {
        const container = containerRef.current
        if (cancelled || !container) return
        if (instanceRef.current) {
          instanceRef.current.update(options)
        } else {
          instanceRef.current = new Styling(options)
          instanceRef.current.append(container)
        }
      })
      .catch((err: unknown) => {
        // A failed chunk load leaves the preview blank; surface it instead of an unhandled rejection.
        console.error('Could not load the QR code renderer', err)
      })
    return () => {
      cancelled = true
    }
  }, [options])

  const text =
    insets.textHeight > 0 ? (
      <div
        className="flex items-center justify-center overflow-hidden font-semibold whitespace-nowrap"
        style={{ height: insets.textHeight, fontSize: Math.round(insets.textHeight * 0.5), color: design.background }}
      >
        {design.frame_text}
      </div>
    ) : null

  return (
    <div
      className={cn('inline-flex flex-col', className)}
      style={{ padding: insets.border, backgroundColor: design.frame === 'none' ? undefined : design.frame_color }}
      data-testid="qr-preview"
    >
      {design.frame === 'text-top' && text}
      <div ref={containerRef} style={{ width: size, height: size }} aria-label="QR code preview" role="img" />
      {design.frame === 'text-bottom' && text}
    </div>
  )
}
