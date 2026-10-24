'use client'

import Link from 'next/link'
import { BarChart2 } from 'lucide-react'
import { QrDownloadMenu } from '@/components/qr/QrDownloadMenu'
import { QrPreview } from '@/components/qr/QrPreview'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { normalizeQrDesign } from '@/lib/qr/design'
import type { QrCode } from '@/types'

/** "Your code is ready!" modal shown after creating a QR Code. */
export function QrReadyDialog({
  open,
  onOpenChange,
  qr,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  qr: QrCode | null
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>Your code is ready!</DialogTitle>
          <DialogDescription>Download it or view its details and scans.</DialogDescription>
        </DialogHeader>
        {qr && (
          <>
            <div className="flex justify-center py-2">
              <QrPreview design={normalizeQrDesign(qr.render_customizations)} data={qr.qr_url} size={160} />
            </div>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <Link
                href={`/dashboard/qr-codes/${qr.id}/customize`}
                className="text-sm font-semibold text-primary hover:underline"
              >
                Customize design
              </Link>
              <div className="flex items-center gap-2">
                <Button variant="outline" asChild>
                  <Link href={`/dashboard/qr-codes/${qr.id}`}>
                    <BarChart2 className="size-4" />
                    View code details
                  </Link>
                </Button>
                <QrDownloadMenu qr={qr} label="Download" />
              </div>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
