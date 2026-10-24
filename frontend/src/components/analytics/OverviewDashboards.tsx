'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { LayoutDashboard } from 'lucide-react'
import { formatDate } from '@/lib/format'
import { useDashboardMutations, useDashboards } from '@/lib/hooks'
import { DashboardNameDialog } from './DashboardNameDialog'

const MAX_TILES = 3
const TILE = 'flex h-32 flex-col rounded-xl border border-border bg-card p-6 transition-colors'

/** "Dashboards" section of the analytics overview: create tile, the 3 newest dashboards, "View all →". */
export function OverviewDashboards() {
  const router = useRouter()
  const { data, isLoading } = useDashboards()
  const { create } = useDashboardMutations()
  const [open, setOpen] = useState(false)
  const dashboards = data?.dashboards.slice(0, MAX_TILES) ?? []

  return (
    <section aria-labelledby="overview-dashboards">
      <div className="flex items-center justify-between">
        <h2 id="overview-dashboards" className="text-xl font-semibold text-foreground">
          Dashboards
        </h2>
        <Link href="/dashboard/analytics/dashboards" className="text-sm font-semibold text-primary hover:underline">
          View all →
        </Link>
      </div>
      <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <button
          type="button"
          onClick={() => setOpen(true)}
          className={`${TILE} items-center justify-center border-dashed font-semibold text-primary hover:bg-accent`}
        >
          + Create dashboard
        </button>
        {isLoading && <div className={`${TILE} animate-pulse`} aria-hidden />}
        {dashboards.map((d) => (
          <Link key={d.id} href={`/dashboard/analytics/dashboards/${d.id}`} className={`${TILE} justify-between hover:border-primary`}>
            <span className="flex min-w-0 items-center gap-2">
              <LayoutDashboard className="size-5 shrink-0 text-primary" aria-hidden />
              <span className="truncate font-semibold text-foreground">{d.name}</span>
            </span>
            <span className="text-sm text-muted-foreground">Modified on {formatDate(d.updated_at)}</span>
          </Link>
        ))}
      </div>
      <DashboardNameDialog
        open={open}
        onOpenChange={setOpen}
        mode="create"
        onSubmit={async (name) => {
          const created = await create.mutateAsync(name)
          router.push(`/dashboard/analytics/dashboards/${created.id}`)
        }}
      />
    </section>
  )
}
