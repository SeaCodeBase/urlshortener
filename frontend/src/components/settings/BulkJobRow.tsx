'use client'

import { useState } from 'react'
import { Download } from 'lucide-react'
import { toast } from 'sonner'
import { StatusChip, type StatusTone } from '@/components/common/StatusChip'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { TableCell, TableRow } from '@/components/ui/table'
import { api } from '@/lib/api'
import { downloadBlob } from '@/lib/download'
import { formatDateTime } from '@/lib/format'
import type { BulkUpload, BulkUploadStatus } from '@/types'

const STATUS: Record<BulkUploadStatus, { label: string; tone: StatusTone }> = {
  queued: { label: 'Queued', tone: 'muted' },
  processing: { label: 'Processing', tone: 'info' },
  done: { label: 'Done', tone: 'success' },
  failed: { label: 'Failed', tone: 'destructive' },
}

/** One bulk upload job: file, status, progress, created and the results download. */
export function BulkJobRow({ upload }: { upload: BulkUpload }) {
  const [downloading, setDownloading] = useState(false)
  const status = STATUS[upload.status] ?? STATUS.queued
  const finished = upload.status === 'done' || upload.status === 'failed'
  const pct = upload.total > 0 ? Math.min(100, Math.round((upload.processed / upload.total) * 100)) : 0

  async function download() {
    setDownloading(true)
    try {
      const blob = await api.downloadBulkResults(upload.id)
      downloadBlob(`bulk-${upload.id}-results.csv`, blob)
    } catch (err) {
      toast.error(err instanceof Error && err.message ? err.message : "Couldn't download the results")
    } finally {
      setDownloading(false)
    }
  }

  return (
    <TableRow>
      <TableCell className="max-w-[240px] truncate font-medium" title={upload.filename}>
        {upload.filename}
      </TableCell>
      <TableCell>
        <StatusChip tone={status.tone} title={upload.status === 'failed' && upload.error ? upload.error : undefined}>
          {status.label}
        </StatusChip>
      </TableCell>
      <TableCell className="min-w-[200px]">
        <div className="text-sm whitespace-nowrap">
          {upload.processed}/{upload.total} · {upload.succeeded} ok · {upload.failed} failed
        </div>
        {upload.status === 'processing' && (
          <Progress value={pct} className="mt-2 h-1.5" aria-label={`${upload.filename} progress`} />
        )}
      </TableCell>
      <TableCell className="text-sm whitespace-nowrap">{formatDateTime(upload.created_at)}</TableCell>
      <TableCell className="text-right">
        <Button variant="secondary" size="sm" disabled={!finished || downloading} onClick={() => void download()}>
          <Download className="size-4" />
          Download results CSV
        </Button>
      </TableCell>
    </TableRow>
  )
}
