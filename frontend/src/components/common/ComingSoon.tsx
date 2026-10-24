import { EmptyState } from './EmptyState'
import { PageHeader } from './PageHeader'

/** Placeholder for sections built in a later phase, so navigation never 404s. */
export function ComingSoon({ title }: { title: string }) {
  return (
    <>
      <PageHeader title={title} />
      <EmptyState title="Coming in a later phase" description={`${title} isn't available yet.`} />
    </>
  )
}
