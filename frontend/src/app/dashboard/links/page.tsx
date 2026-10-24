import { Suspense } from 'react'
import Link from 'next/link'
import { PageHeader } from '@/components/common/PageHeader'
import { LinksList } from '@/components/links/LinksList'
import { Button } from '@/components/ui/button'

export default function LinksPage() {
  return (
    <div>
      <PageHeader
        title="Links"
        actions={
          <Button asChild>
            <Link href="/dashboard/links/create">Create link</Link>
          </Button>
        }
      />
      {/* useSearchParams needs a Suspense boundary for static rendering. */}
      <Suspense>
        <LinksList />
      </Suspense>
    </div>
  )
}
