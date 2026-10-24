import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'

export default function WebhooksPage() {
  return (
    <>
      <PageHeader title="Webhooks" />
      <EmptyState title="Coming in a later phase" description="Webhooks arrive in Phase 5." />
    </>
  )
}
