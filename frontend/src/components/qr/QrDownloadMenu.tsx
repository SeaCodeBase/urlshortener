'use client'

import { Download } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { downloadQr, type QrImageFormat } from '@/lib/qr/render'
import type { QrCode } from '@/types'

const FORMATS: { value: QrImageFormat; label: string }[] = [
  { value: 'png', label: 'PNG' },
  { value: 'svg', label: 'SVG' },
  { value: 'jpeg', label: 'JPG' },
]

/** Download icon button with a PNG / SVG / JPG menu; renders the stored design client-side. */
export function QrDownloadMenu({ qr }: { qr: QrCode }) {
  function download(format: QrImageFormat) {
    downloadQr(qr, format).catch(() => toast.error("Couldn't download the QR Code"))
  }

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="secondary" size="icon-sm" aria-label="Download" className="size-8">
          <Download className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-32">
        {FORMATS.map((f) => (
          <DropdownMenuItem key={f.value} onSelect={() => download(f.value)}>
            {f.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
