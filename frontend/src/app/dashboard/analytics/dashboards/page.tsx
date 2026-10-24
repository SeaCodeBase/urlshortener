'use client'

import { Suspense, useState } from 'react'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { DashboardRow } from '@/components/analytics/DashboardRow'
import { DashboardNameDialog } from '@/components/analytics/DashboardNameDialog'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useDashboardMutations, useDashboards } from '@/lib/hooks'
import type { DashboardSort } from '@/types'

const SORT_OPTIONS: { value: DashboardSort; label: string }[] = [
  { value: 'created', label: 'Date created' },
  { value: 'modified', label: 'Date modified' },
  { value: 'name', label: 'Name' },
]

function parseSort(raw: string | null): DashboardSort {
  return SORT_OPTIONS.some((o) => o.value === raw) ? (raw as DashboardSort) : 'created'
}

function RowSkeleton() {
  return (
    <div className="flex items-center gap-4 rounded-xl bg-card p-6" aria-hidden>
      <Skeleton className="size-10 rounded-full" />
      <div className="flex flex-1 flex-col gap-2">
        <Skeleton className="h-5 w-1/3" />
        <Skeleton className="h-4 w-1/2" />
      </div>
    </div>
  )
}

function AllDashboards() {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const sort = parseSort(searchParams.get('sort'))
  const query = useDashboards(sort)
  const { create } = useDashboardMutations()
  const [createOpen, setCreateOpen] = useState(false)
  const dashboards = query.data?.dashboards ?? []
  const sortLabel = SORT_OPTIONS.find((o) => o.value === sort)?.label

  function setSort(next: string) {
    router.replace(next === 'created' ? pathname : `${pathname}?sort=${next}`, { scroll: false })
  }

  const createButton = (
    <Button type="button" onClick={() => setCreateOpen(true)}>
      Create new dashboard
    </Button>
  )

  let body
  if (query.isPending) {
    body = (
      <div className="flex flex-col gap-4" aria-busy="true" aria-label="Loading dashboards">
        <RowSkeleton />
        <RowSkeleton />
        <RowSkeleton />
      </div>
    )
  } else if (query.isError) {
    body = (
      <EmptyState
        title="Couldn't load dashboards"
        description="Something went wrong. Please try again."
        action={
          <Button type="button" variant="secondary" onClick={() => void query.refetch()}>
            Retry
          </Button>
        }
      />
    )
  } else if (dashboards.length === 0) {
    body = <EmptyState title="No dashboards yet" description="Create a dashboard to track your engagements." action={createButton} />
  } else {
    body = (
      <div className="flex flex-col gap-4">
        {dashboards.map((d) => (
          <DashboardRow key={d.id} dashboard={d} />
        ))}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4 pb-12">
      <div className="flex flex-wrap items-center gap-2">
        <Select value={sort} onValueChange={setSort}>
          <SelectTrigger aria-label="Sort by" className="min-w-52 bg-card">
            <SelectValue>Sort by: {sortLabel}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            {SORT_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Tooltip>
          <TooltipTrigger asChild>
            <span tabIndex={0}>
              <Button type="button" variant="secondary" disabled>
                Add filters
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>Coming in a later phase</TooltipContent>
        </Tooltip>
        <div className="ml-auto">{createButton}</div>
      </div>
      {body}
      <DashboardNameDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        mode="create"
        onSubmit={async (name) => {
          const created = await create.mutateAsync(name)
          router.push(`/dashboard/analytics/dashboards/${created.id}`)
        }}
      />
    </div>
  )
}

export default function DashboardsPage() {
  return (
    <div>
      <PageHeader title="Dashboards" backHref="/dashboard/analytics" />
      {/* useSearchParams needs a Suspense boundary for static rendering. */}
      <Suspense>
        <AllDashboards />
      </Suspense>
    </div>
  )
}
