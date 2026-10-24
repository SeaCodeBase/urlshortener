'use client'

import { Suspense, useRef, useState } from 'react'
import NextLink from 'next/link'
import { useParams, usePathname, useRouter, useSearchParams } from 'next/navigation'
import { Download, Pencil } from 'lucide-react'
import { toast } from 'sonner'
import { DashboardControls, type DashboardCompare, type DayRange } from '@/components/analytics/DashboardControls'
import { ModuleGrid } from '@/components/analytics/ModuleGrid'
import { slugify } from '@/components/analytics/dashboardCsv'
import { rangeFor } from '@/components/analytics/modules'
import { EmptyState } from '@/components/common/EmptyState'
import { parseAnalyticsUrlState, unitFor } from '@/components/links/LinkAnalytics'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { ApiError, api } from '@/lib/api'
import { downloadBlob } from '@/lib/download'
import { useDashboard, useDashboardData, useDashboardMutations } from '@/lib/hooks'
import type { Dashboard, DashboardDataQuery, DashboardPatch } from '@/types'

const BACK_HREF = '/dashboard/analytics/dashboards'

const errorMessage = (err: unknown, fallback: string) => (err instanceof ApiError ? err.message : fallback)

function BackLink() {
  return (
    <NextLink href={BACK_HREF} className="text-sm font-semibold text-primary hover:underline">
      ← Dashboards
    </NextLink>
  )
}

function GridSkeleton() {
  return (
    <div className="grid gap-4 lg:grid-cols-2" aria-busy="true" aria-label="Loading dashboard">
      <Skeleton className="h-36 rounded-xl" />
      <Skeleton className="h-36 rounded-xl" />
      <Skeleton className="h-80 rounded-xl lg:col-span-2" />
      <Skeleton className="h-72 rounded-xl" />
      <Skeleton className="h-72 rounded-xl" />
    </div>
  )
}

function PageSkeleton() {
  return (
    <div className="space-y-6 pt-8">
      <Skeleton className="h-10 w-80" />
      <Skeleton className="h-10 w-full max-w-xl" />
      <GridSkeleton />
    </div>
  )
}

/** Click to edit; Enter or blur saves, Escape cancels. */
function EditableTitle({ name, onSave }: { name: string; onSave: (name: string) => Promise<void> }) {
  const [draft, setDraft] = useState<string | null>(null)
  const done = useRef(false)

  if (draft == null) {
    return (
      <h1 className="min-w-0">
        <button
          type="button"
          onClick={() => {
            done.current = false
            setDraft(name)
          }}
          aria-label={`Rename dashboard ${name}`}
          className="group inline-flex max-w-full items-center gap-2 rounded text-left text-[32px] leading-10 font-semibold tracking-[0.32px] text-foreground hover:bg-accent"
        >
          <span className="truncate">{name}</span>
          <Pencil className="size-5 shrink-0 text-muted-foreground opacity-0 group-hover:opacity-100" aria-hidden />
        </button>
      </h1>
    )
  }

  const finish = async (save: boolean) => {
    if (done.current) return
    done.current = true
    const next = draft.trim()
    setDraft(null)
    if (save && next && next !== name) await onSave(next)
  }

  return (
    <Input
      autoFocus
      aria-label="Dashboard name"
      maxLength={255}
      value={draft}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => finish(true)}
      onKeyDown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault()
          finish(true)
        } else if (e.key === 'Escape') {
          e.preventDefault()
          finish(false)
        }
      }}
      className="h-12 max-w-xl text-2xl font-semibold"
    />
  )
}

function DashboardView({ dashboard }: { dashboard: Dashboard }) {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const { update } = useDashboardMutations()
  const [exporting, setExporting] = useState(false)

  const urlState = parseAnalyticsUrlState(searchParams)
  const customRange: DayRange | null = urlState.from && urlState.to ? { from: urlState.from, to: urlState.to } : null
  const range = customRange ?? rangeFor(dashboard.date_range)
  const compare: DashboardCompare = searchParams.get('compare') === 'none' ? 'none' : 'previous'
  const unit = unitFor(range.from, range.to)
  const query: DashboardDataQuery = { ...range, compare, unit }
  const { data, isLoading, isError, isFetching, refetch } = useDashboardData(dashboard.id, query)

  function setUrl(patch: { range?: DayRange | null; compare?: DashboardCompare }) {
    const sp = new URLSearchParams(searchParams.toString())
    if (patch.range !== undefined) {
      if (patch.range) {
        sp.set('from', patch.range.from)
        sp.set('to', patch.range.to)
      } else {
        sp.delete('from')
        sp.delete('to')
      }
    }
    if (patch.compare) {
      if (patch.compare === 'none') sp.set('compare', 'none')
      else sp.delete('compare')
    }
    const qs = sp.toString()
    router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false })
  }

  const save = (patch: DashboardPatch) => update.mutateAsync({ id: dashboard.id, patch })

  async function rename(name: string) {
    try {
      await save({ name })
      toast.success('Dashboard renamed')
    } catch (err) {
      toast.error(errorMessage(err, "Couldn't rename the dashboard"))
    }
  }

  async function exportCsv() {
    setExporting(true)
    try {
      const blob = await api.exportDashboard(dashboard.id, { ...range, compare })
      downloadBlob(`${slugify(dashboard.name)}-${range.from}-${range.to}.zip`, blob)
    } catch (err) {
      toast.error(errorMessage(err, "Couldn't export the dashboard"))
    } finally {
      setExporting(false)
    }
  }

  return (
    <div className="space-y-6 pb-12">
      <div className="space-y-3 pt-8">
        <BackLink />
        <div className="flex items-center justify-between gap-4">
          <EditableTitle name={dashboard.name} onSave={rename} />
          <Button type="button" variant="secondary" className="shrink-0" disabled={exporting} onClick={exportCsv}>
            <Download className="size-4" />
            Download CSV
          </Button>
        </div>
      </div>

      <DashboardControls
        dashboard={dashboard}
        customRange={customRange}
        range={range}
        compare={compare}
        onPreset={(date_range) => {
          setUrl({ range: null })
          save({ date_range }).catch((err) => toast.error(errorMessage(err, "Couldn't change the date range")))
        }}
        onCustomRange={(r) => setUrl({ range: r })}
        onCompareChange={(c) => setUrl({ compare: c })}
        onFilterApply={async (filter) => {
          await save({ filter })
        }}
      />

      {!data && isError ? (
        <EmptyState
          title="Couldn't load dashboard data"
          description="Something went wrong while loading the modules."
          action={
            <Button type="button" variant="secondary" disabled={isFetching} onClick={() => refetch()}>
              Retry
            </Button>
          }
        />
      ) : !data || isLoading ? (
        <GridSkeleton />
      ) : (
        <ModuleGrid
          layout={dashboard.layout}
          data={data}
          compare={compare === 'previous' && data.compare}
          unit={data.unit}
          onLayoutChange={(layout) => save({ layout })}
        />
      )}
    </div>
  )
}

function DashboardPageInner() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)
  const valid = Number.isInteger(id) && id > 0
  const { data: dashboard, isLoading, isError, error, isFetching, refetch } = useDashboard(valid ? id : null)

  if (!valid || (error instanceof ApiError && error.status === 404)) {
    return (
      <div className="pt-8">
        <EmptyState
          title="Dashboard not found"
          description="It may have been deleted, or it belongs to another group."
          action={
            <Button asChild>
              <NextLink href={BACK_HREF}>Back to Dashboards</NextLink>
            </Button>
          }
        />
      </div>
    )
  }
  if (isLoading) return <PageSkeleton />
  if (isError || !dashboard) {
    return (
      <div className="space-y-4 pt-8">
        <BackLink />
        <EmptyState
          title="Couldn't load the dashboard"
          description={errorMessage(error, 'Something went wrong.')}
          action={
            <Button type="button" variant="secondary" disabled={isFetching} onClick={() => refetch()}>
              Retry
            </Button>
          }
        />
      </div>
    )
  }
  return <DashboardView dashboard={dashboard} />
}

/** A dashboard (spec §5.8): controls, reorderable modules, per-module visualization and CSV, zip export. */
export default function DashboardPage() {
  return (
    // useSearchParams needs a Suspense boundary.
    <Suspense fallback={<PageSkeleton />}>
      <DashboardPageInner />
    </Suspense>
  )
}
