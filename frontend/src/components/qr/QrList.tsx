'use client'

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import NextLink from 'next/link'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
import { EmptyState } from '@/components/common/EmptyState'
import { BulkTagDialog } from '@/components/links/BulkTagDialog'
import { LinksToolbar } from '@/components/links/LinksToolbar'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { api } from '@/lib/api'
import { downloadBlob } from '@/lib/download'
import { useBulkQrCodes, useQrCodes, useTags } from '@/lib/hooks'
import { QrCard } from './QrCard'
import { QrFilterBar } from './QrFilterBar'
import { hasActiveQrQuery, parseQrUrlState, qrUrlStateToSearch, toQrApiParams, type QrUrlState } from './qrUrlState'

const PAGE_SIZE = 20
const EMPTY_SELECTION: ReadonlySet<number> = new Set()

function QrCardSkeleton() {
  return (
    <div className="flex gap-4 rounded-xl bg-card pt-4 pr-4 pb-6 pl-8" aria-hidden>
      <Skeleton className="size-[120px] rounded" />
      <div className="flex flex-1 flex-col gap-2">
        <Skeleton className="h-6 w-1/3" />
        <Skeleton className="h-5 w-1/5" />
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
      You&apos;ve reached the end of your QR Codes
      <span className="h-px flex-1 bg-border" />
    </div>
  )
}

/** The QR Codes list: URL-driven filters, infinite scroll, selection and bulk actions (same structure as LinksList). */
export function QrList() {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const state = useMemo(() => parseQrUrlState(searchParams), [searchParams])
  const apiParams = useMemo(() => toQrApiParams(state), [state])
  const filterKey = JSON.stringify(apiParams)

  const update = useCallback(
    (patch: Partial<QrUrlState>) => {
      const qs = qrUrlStateToSearch({ ...state, ...patch })
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false })
    },
    [state, router, pathname],
  )

  const query = useQrCodes({ ...apiParams, limit: PAGE_SIZE })
  const { data: tagSuggestions = [] } = useTags('qr')
  const bulk = useBulkQrCodes()
  const codes = useMemo(() => query.data?.pages.flatMap((p) => p.qr_codes) ?? [], [query.data])

  // Selection is tied to the filter set it was made under, so changing filters clears it.
  const [selection, setSelection] = useState<{ key: string; ids: ReadonlySet<number> }>({
    key: filterKey,
    ids: EMPTY_SELECTION,
  })
  const selected = selection.key === filterKey ? selection.ids : EMPTY_SELECTION
  const setSelected = (ids: ReadonlySet<number>) => setSelection({ key: filterKey, ids })
  const clearSelection = () => setSelected(EMPTY_SELECTION)
  const selectedIds = codes.filter((c) => selected.has(c.id)).map((c) => c.id)
  const allSelected = codes.length > 0 && selectedIds.length === codes.length

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
  const noun = n === 1 ? 'QR Code' : 'QR Codes'

  function runHide() {
    return bulk.mutateAsync({ action: hidden ? 'unarchive' : 'archive', ids: selectedIds }).then(
      () => {
        toast.success(hidden ? 'QR Codes unhidden' : 'QR Codes hidden')
        setHideOpen(false)
        clearSelection()
      },
      () => toast.error(hidden ? "Couldn't unhide QR Codes" : "Couldn't hide QR Codes"),
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
      downloadBlob('qr-codes.csv', await api.exportQrCodes(apiParams))
      toast.success('Export downloaded')
      clearSelection()
    } catch {
      toast.error("Couldn't export QR Codes")
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
  }, [hasNextPage, isFetchingNextPage, fetchNextPage, codes.length])

  const filtering = hasActiveQrQuery(state)

  let body
  if (query.isPending) {
    body = (
      <div className="flex flex-col gap-4" aria-busy="true" aria-label="Loading QR Codes">
        <QrCardSkeleton />
        <QrCardSkeleton />
        <QrCardSkeleton />
      </div>
    )
  } else if (query.isError) {
    body = (
      <EmptyState
        title="Couldn't load QR Codes"
        description="Something went wrong. Please try again."
        action={
          <Button type="button" variant="secondary" onClick={() => void query.refetch()}>
            Try again
          </Button>
        }
      />
    )
  } else if (codes.length === 0 && !filtering && !hidden) {
    body = (
      <EmptyState
        title="No QR Codes yet"
        description="Create your first QR Code."
        action={
          <Button asChild>
            <NextLink href="/dashboard/qr-codes/create">Create QR Code</NextLink>
          </Button>
        }
      />
    )
  } else if (codes.length === 0) {
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
              {hidden ? 'try searching active codes instead.' : 'try searching hidden codes instead.'}
            </button>
          </>
        }
      />
    )
  } else {
    body = (
      <>
        <div className="flex flex-col gap-4">
          {codes.map((qr) => (
            <QrCard key={qr.id} qr={qr} selected={selected.has(qr.id)} onSelect={(on) => toggleOne(qr.id, on)} />
          ))}
        </div>
        {hasNextPage ? (
          <div ref={sentinel} className="flex flex-col gap-4 pt-4">
            {isFetchingNextPage && <QrCardSkeleton />}
          </div>
        ) : (
          <EndOfList />
        )}
      </>
    )
  }

  return (
    <div className="flex flex-col gap-4 pb-8">
      <QrFilterBar state={state} onChange={update} />
      <LinksToolbar
        noun="codes"
        showViews={false}
        selectedCount={n}
        allSelected={allSelected}
        selectAllDisabled={codes.length === 0}
        onToggleAll={(on) => setSelected(on ? new Set(codes.map((c) => c.id)) : EMPTY_SELECTION)}
        onExport={() => void runExport()}
        exporting={exporting}
        onHide={() => setHideOpen(true)}
        onTag={() => setTagOpen(true)}
        show={state.show}
        onShowChange={(show) => update({ show })}
      />
      {body}

      <ConfirmDialog
        open={hideOpen}
        onOpenChange={setHideOpen}
        title={hidden ? `Unhide ${n} ${noun}?` : `Hide ${n} ${noun}?`}
        description={
          hidden
            ? 'Unhidden QR Codes show in your list and analytics again.'
            : 'Hidden QR Codes keep working but no longer show in your list or analytics.'
        }
        confirmLabel={hidden ? 'Unhide QR Codes' : 'Hide QR Codes'}
        loading={bulk.isPending}
        onConfirm={runHide}
      />
      <BulkTagDialog
        open={tagOpen}
        onOpenChange={setTagOpen}
        noun="codes"
        count={n}
        suggestions={tagSuggestions}
        saving={bulk.isPending}
        onSave={runTag}
      />
    </div>
  )
}
