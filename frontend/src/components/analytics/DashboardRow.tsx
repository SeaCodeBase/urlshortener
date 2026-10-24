import Link from 'next/link'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { formatDate } from '@/lib/format'
import type { Dashboard, DashboardFilter } from '@/types'
import { DashboardActionsMenu } from './DashboardActionsMenu'
import { DATE_RANGE_OPTIONS } from './modules'

/** "Showing data for …" line describing a dashboard's link / QR filter. */
export function filterSummary(filter: DashboardFilter): string {
  switch (filter.type) {
    case 'links':
      return `Showing data for ${filter.ids.length} ${filter.ids.length === 1 ? 'link' : 'links'}`
    case 'qr':
      return `Showing data for ${filter.ids.length} ${filter.ids.length === 1 ? 'QR Code' : 'QR Codes'}`
    default:
      return 'Showing data for all links and QR Codes'
  }
}

/** One row of the "All dashboards" list. */
export function DashboardRow({ dashboard }: { dashboard: Dashboard }) {
  const initial = dashboard.created_by.name.trim().charAt(0).toUpperCase() || '?'
  const rangeLabel = DATE_RANGE_OPTIONS.find((o) => o.value === dashboard.date_range)?.label ?? dashboard.date_range

  return (
    <div className="flex items-center gap-4 rounded-xl bg-card p-6">
      <Avatar className="size-10 shrink-0">
        <AvatarFallback className="bg-primary/10 font-semibold text-primary">{initial}</AvatarFallback>
      </Avatar>
      <div className="flex min-w-0 flex-1 flex-col gap-1 md:flex-row md:items-center md:gap-6">
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <Link
            href={`/dashboard/analytics/dashboards/${dashboard.id}`}
            className="truncate text-base font-semibold text-foreground hover:text-primary hover:underline"
          >
            {dashboard.name}
          </Link>
          <p className="flex flex-wrap gap-x-3 text-sm text-muted-foreground">
            <span>Modified on {formatDate(dashboard.updated_at)}</span>
            <span>Created by {dashboard.created_by.name}</span>
          </p>
        </div>
        <div className="flex min-w-0 flex-col gap-1 text-sm md:w-72">
          <span className="font-medium text-foreground">{rangeLabel}</span>
          <span className="text-muted-foreground">{filterSummary(dashboard.filter)}</span>
        </div>
      </div>
      <DashboardActionsMenu dashboard={dashboard} />
    </div>
  )
}
