'use client'

import Link from 'next/link'
import { BarChart2, Copy, QrCode } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { formatShortUrl } from '@/lib/format'

/** "Your link is ready!" modal shown after a quick create. */
export function LinkReadyDialog({
  open,
  onOpenChange,
  link,
  showCreateQr = false,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  link: { id: number; short_url: string } | null
  /** Adds a "Create QR Code" button (the "Also create a QR Code" checkbox was on). */
  showCreateQr?: boolean
}) {
  async function copy() {
    if (!link) return
    try {
      await navigator.clipboard.writeText(link.short_url)
      toast.success('Copied to clipboard')
    } catch {
      toast.error("Couldn't copy to clipboard")
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Your link is ready!</DialogTitle>
          <DialogDescription>Copy the link below to share it or view its details.</DialogDescription>
        </DialogHeader>
        {link && (
          <>
            <div className="rounded-lg bg-sidebar-active-bg px-4 py-5 text-center text-base font-semibold break-all text-primary">
              {formatShortUrl(link.short_url)}
            </div>
            <div className="flex flex-wrap justify-end gap-2">
              {showCreateQr && (
                <Button variant="outline" asChild>
                  <Link href={`/dashboard/qr-codes/create?link=${link.id}`}>
                    <QrCode className="size-4" />
                    Create QR Code
                  </Link>
                </Button>
              )}
              <Button variant="outline" asChild>
                <Link href={`/dashboard/links/${link.id}`}>
                  <BarChart2 className="size-4" />
                  View link details
                </Link>
              </Button>
              <Button type="button" onClick={copy}>
                <Copy className="size-4" />
                Copy link
              </Button>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
