import { Suspense } from 'react'
import Link from 'next/link'
import { PageHeader } from '@/components/common/PageHeader'
import { QrList } from '@/components/qr/QrList'
import { Button } from '@/components/ui/button'

export default function QrCodesPage() {
  return (
    <div>
      <PageHeader
        title="QR Codes"
        actions={
          <Button asChild>
            <Link href="/dashboard/qr-codes/create">Create QR Code</Link>
          </Button>
        }
      />
      {/* useSearchParams needs a Suspense boundary for static rendering. */}
      <Suspense>
        <QrList />
      </Suspense>
    </div>
  )
}
