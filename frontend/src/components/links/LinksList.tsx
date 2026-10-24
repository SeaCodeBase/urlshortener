'use client'

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import NextLink from 'next/link'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
import { EmptyState } from '@/components/common/EmptyState'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { api } from '@/lib/api'
import { downloadBlob } from '@/lib/download'
import { useBulkLinks, useLinks, useTags } from '@/lib/hooks'
import { cn } from '@/lib/utils'
import { BulkTagDialog } from './BulkTagDialog'
import { LinkCard } from './LinkCard'
import { LinkCardCompact } from './LinkCardCompact'
import { LinksFilterBar } from './LinksFilterBar'
import { LinksToolbar } from './LinksToolbar'
import {
  hasActiveQuery,
  linksUrlStateToSearch,
  parseLinksUrlState,
  toApiParams,
  type LinksUrlState,
} from './linksUrlState'
import { useLinksView } from './useLinksView'

const PAGE_SIZE = 20
const EMPTY_SELECTION: ReadonlySet<number> = new Set()

function LinkCardSkeleton() {
  return (
    <div className="flex gap-4 rounded-xl bg-card pt-4 pr-4 pb-6 pl-8" aria-hidden>
      <Skeleton className="size-8 rounded-full" />
      <div className="flex flex-1 flex-col gap-2">
        <Skeleton className="h-6 w-1/3" />
        <Skeleton className="h-5 w-1/4" />
        <Skeleton className="h-4 w-1/2" />
        <Skeleton className="mt-3 h-4 w-2/5" />
      </div>
    </div>
  )
}

function EndOfList() {
  return (
    <div className="flex items-center gap-4 py-6 text-sm text-muted-foreground">
      <span className="h-px flex-1 bg-border" />
      You&apos;ve reached the end of your links
      <span className="h-px flex-1 bg-border" />
    </div>
  )
}

/** The links list: URL-driven filters, infinite scroll, selection and bulk actions (spec §5.2). */
export function LinksList() {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const state = useMemo(() => parseLinksUrlState(searchParams), [searchParams])
  const apiParams = useMemo(() => toApiParams(state), [state])
  const filterKey = JSON.stringify(apiParams)

  const update = useCallback(
    (patch: Partial<LinksUrlState>) => {
      const qs = linksUrlStateToSearch({ ...state, ...patch })
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false })
    },
    [state, router, pathname],
  )

  const [view, setView] = useLinksView()
  const query = useLinks({ ...apiParams, limit: PAGE_SIZE })
  const { data: tagSuggestions = [] } = useTags()
  const bulk = useBulkLinks()
  const links = useMemo(() => query.data?.pages.flatMap((p) => p.links) ?? [], [query.data])

  // Selection is tied to the filter set it was made under, so changing filters clears it.
  const [selection, setSelection] = useState<{ key: string; ids: ReadonlySet<number> }>({
    key: filterKey,
    ids: EMPTY_SELECTION,
  })
  const selected = selection.key === filterKey ? selection.ids : EMPTY_SELECTION
  const setSelected = (ids: ReadonlySet<number>) => setSelection({ key: filterKey, ids })
  const clearSelection = () => setSelected(EMPTY_SELECTION)
  const selectedIds = links.filter((l) => selected.has(l.id)).map((l) => l.id)
  const allSelected = links.length > 0 && selectedIds.length === links.length

  function toggleOne(id: number, on: boolean) {
    const next = new Set(selected)
    if (on) next.add(id)
    else next.delete(id)
    setSelected(next)
  }

  const [hideOpen, setHideOpen] = useState(false)
  const [tagOpen, setTagOpen] = useState(false)
  const [exporting, setExporting] = useState(false)
  const hidden = state.show === 'hidden'
  const n = selectedIds.length
  const noun = n === 1 ? 'link' : 'links'

  function runHide() {
    return bulk.mutateAsync({ action: hidden ? 'unarchive' : 'archive', ids: selectedIds }).then(
      () => {
        toast.success(hidden ? 'Links unhidden' : 'Links hidden')
        setHideOpen(false)
        clearSelection()
      },
      () => toast.error(hidden ? "Couldn't unhide links" : "Couldn't hide links"),
    )
  }

  function runTag(tags: string[]) {
    bulk.mutate(
      { action: 'edit_tags', ids: selectedIds, add_tags: tags },
      {
        onSuccess: () => {
          toast.success('Tags added')
          setTagOpen(false)
          clearSelection()
        },
        onError: () => toast.error("Couldn't add tags"),
      },
    )
  }

  async function runExport() {
    setExporting(true)
    try {
      downloadBlob('links.csv', await api.exportLinks(apiParams))
      toast.success('Export downloaded')
      clearSelection()
    } catch {
      toast.error("Couldn't export links")
    } finally {
      setExporting(false)
    }
  }

  // Infinite scroll: load the next page when the sentinel scrolls into view.
  const sentinel = useRef<HTMLDivElement | null>(null)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query
  useEffect(() => {
    const el = sentinel.current
    if (!el || !hasNextPage || typeof IntersectionObserver === 'undefined') return
    const io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting) && !isFetchingNextPage) void fetchNextPage()
      },
      { rootMargin: '400px' },
    )
    io.observe(el)
    return () => io.disconnect()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage, links.length])

  const filtering = hasActiveQuery(state)
  const loading = query.isPending

  let body
  if (loading) {
    body = (
      <div className="flex flex-col gap-4" aria-busy="true" aria-label="Loading links">
        <LinkCardSkeleton />
        <LinkCardSkeleton />
        <LinkCardSkeleton />
      </div>
    )
  } else if (query.isError) {
    body = (
      <EmptyState
        title="Couldn't load links"
        description="Something went wrong. Please try again."
        action={
          <Button type="button" variant="secondary" onClick={() => void query.refetch()}>
            Try again
          </Button>
        }
      />
    )
  } else if (links.length === 0 && !filtering && !hidden) {
    body = (
      <EmptyState
        title="No links yet"
        description="Create your first short link."
        action={
          <Button asChild>
            <NextLink href="/dashboard/links/create">Create link</NextLink>
          </Button>
        }
      />
    )
  } else if (links.length === 0) {
    body = (
      <EmptyState
        title="No results found"
        description={
          <>
            Try adjusting your search, filters, or{' '}
            <button
              type="button"
              className="font-medium text-primary hover:underline"
              onClick={() => update({ show: hidden ? 'active' : 'hidden' })}
            >
              {hidden ? 'try searching active links instead.' : 'try searching hidden links instead.'}
            </button>
          </>
        }
      />
    )
  } else {
    body = (
      <>
        <div
          className={cn(
            view === 'grid' ? 'grid grid-cols-1 gap-4 lg:grid-cols-2 xl:grid-cols-3' : 'flex flex-col',
            view === 'compact' ? 'gap-2' : 'gap-4',
          )}
        >
          {links.map((link) =>
            view === 'compact' ? (
              <LinkCardCompact
                key={link.id}
                link={link}
                selected={selected.has(link.id)}
                onSelectedChange={(on) => toggleOne(link.id, on)}
              />
            ) : (
              <LinkCard
                key={link.id}
                link={link}
                layout={view === 'grid' ? 'grid' : 'default'}
                selected={selected.has(link.id)}
                onSelectedChange={(on) => toggleOne(link.id, on)}
              />
            ),
          )}
        </div>
        {hasNextPage ? (
          <div ref={sentinel} className="flex flex-col gap-4 pt-4">
            {isFetchingNextPage && <LinkCardSkeleton />}
          </div>
        ) : (
          <EndOfList />
        )}
      </>
    )
  }

  return (
    <div className="flex flex-col gap-4 pb-8">
      <LinksFilterBar state={state} onChange={update} />
      <LinksToolbar
        selectedCount={n}
        allSelected={allSelected}
        selectAllDisabled={links.length === 0}
        onToggleAll={(on) => setSelected(on ? new Set(links.map((l) => l.id)) : EMPTY_SELECTION)}
        onExport={() => void runExport()}
        exporting={exporting}
        onHide={() => setHideOpen(true)}
        onTag={() => setTagOpen(true)}
        show={state.show}
        onShowChange={(show) => update({ show })}
        view={view}
        onViewChange={setView}
      />
      {body}

      <ConfirmDialog
        open={hideOpen}
        onOpenChange={setHideOpen}
        title={hidden ? `Unhide ${n} ${noun}?` : `Hide ${n} ${noun}?`}
        description={
          hidden
            ? 'Unhidden links show in your list and analytics again.'
            : 'Hidden links keep working but no longer show in your list or analytics.'
        }
        confirmLabel={hidden ? 'Unhide links' : 'Hide links'}
        loading={bulk.isPending}
        onConfirm={runHide}
      />
      <BulkTagDialog
        open={tagOpen}
        onOpenChange={setTagOpen}
        count={n}
        suggestions={tagSuggestions}
        saving={bulk.isPending}
        onSave={runTag}
      />
    </div>
  )
}
