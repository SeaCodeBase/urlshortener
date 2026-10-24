'use client'

import { useState } from 'react'
import NextLink from 'next/link'
import { useParams, useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { QrDesigner } from '@/components/qr/QrDesigner'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useQrCode, useUpdateQrCode } from '@/lib/hooks'
import { normalizeQrDesign } from '@/lib/qr/design'
import type { QrCode, QrDesign } from '@/types'

function CustomizeQrForm({ qr }: { qr: QrCode }) {
  const router = useRouter()
  const updateQr = useUpdateQrCode()
  const [design, setDesign] = useState<QrDesign>(() => normalizeQrDesign(qr.render_customizations))
  const detailsHref = `/dashboard/qr-codes/${qr.id}`

  async function save() {
    if (updateQr.isPending) return
    try {
      await updateQr.mutateAsync({ id: qr.id, data: { render_customizations: design } })
      toast.success('Design saved')
      router.push(detailsHref)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Couldn't save the design")
    }
  }

  return (
    <div>
      <div className="pb-24">
        <QrDesigner design={design} onChange={setDesign} data={qr.qr_url} />
      </div>
      <div className="sticky bottom-0 -mx-4 border-t border-border bg-background px-4 py-3">
        <div className="flex justify-end gap-3">
          <Button type="button" variant="secondary" asChild>
            <NextLink href={detailsHref}>Cancel</NextLink>
          </Button>
          <Button type="button" onClick={save} disabled={updateQr.isPending}>
            {updateQr.isPending ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </div>
    </div>
  )
}

export default function CustomizeQrCodePage() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)
  const { data: qr, isLoading, isError } = useQrCode(Number.isFinite(id) ? id : null)

  return (
    <div className="mx-auto w-full max-w-[1040px]">
      <PageHeader
        title="Customize design"
        backHref={Number.isFinite(id) ? `/dashboard/qr-codes/${id}` : '/dashboard/qr-codes'}
      />
      {isLoading ? (
        <Skeleton className="h-96 rounded-xl" />
      ) : isError || !qr ? (
        <EmptyState
          title="QR Code not found"
          description="It may have been deleted, or it belongs to another group."
          action={
            <Button asChild>
              <NextLink href="/dashboard/qr-codes">Back to QR Codes</NextLink>
            </Button>
          }
        />
      ) : (
        <CustomizeQrForm key={qr.id} qr={qr} />
      )}
    </div>
  )
}
