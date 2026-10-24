'use client'

import NextLink from 'next/link'
import { useRouter } from 'next/navigation'
import { ArrowLeft, Palette, Pencil } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { hostOf } from '@/lib/format'
import type { QrCode } from '@/types'
import { QrActionsMenu } from './QrActionsMenu'
import { QrDownloadMenu } from './QrDownloadMenu'

/** QR Code details title row: back, title; ⋯ menu, Edit, Customize and Download on the right. */
export function QrDetailsHeader({ qr }: { qr: QrCode }) {
  const router = useRouter()
  const title = qr.title || hostOf(qr.original_url)

  return (
    <div className="flex items-center justify-between gap-4 pt-8 pb-6">
      <div className="flex min-w-0 items-center gap-3">
        <NextLink
          href="/dashboard/qr-codes"
          aria-label="Back to QR Codes"
          className="inline-flex size-8 shrink-0 items-center justify-center rounded text-foreground hover:bg-accent"
        >
          <ArrowLeft className="size-5" />
        </NextLink>
        <h1 className="truncate text-[32px] leading-10 font-semibold tracking-[0.32px] text-foreground">{title}</h1>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <QrActionsMenu qr={qr} triggerClassName="size-10" onDeleted={() => router.push('/dashboard/qr-codes')} />
        <Button variant="secondary" size="icon" aria-label="Edit" asChild>
          <NextLink href={`/dashboard/qr-codes/${qr.id}/edit`}>
            <Pencil className="size-5" />
          </NextLink>
        </Button>
        <Button variant="secondary" size="icon" aria-label="Customize" asChild>
          <NextLink href={`/dashboard/qr-codes/${qr.id}/customize`}>
            <Palette className="size-5" />
          </NextLink>
        </Button>
        <QrDownloadMenu qr={qr} label="Download" />
      </div>
    </div>
  )
}
