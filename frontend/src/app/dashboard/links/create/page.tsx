import Link from 'next/link'
import { PageHeader } from '@/components/common/PageHeader'
import { LinkForm } from '@/components/links/LinkForm'

export default function CreateLinkPage() {
  return (
    <div className="mx-auto w-full max-w-[720px]">
      <PageHeader
        title="Create a new link"
        actions={
          <Link href="/dashboard/settings/bulk-upload" className="text-base font-semibold text-primary hover:underline">
            Bulk upload
          </Link>
        }
      />
      <LinkForm mode="create" />
    </div>
  )
}
