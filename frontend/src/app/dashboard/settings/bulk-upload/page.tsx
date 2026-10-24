import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'

export default function BulkUploadPage() {
  return (
    <>
      <PageHeader title="Bulk upload" />
      <EmptyState title="Coming in a later phase" description="Bulk upload arrives in Phase 5." />
    </>
  )
}
