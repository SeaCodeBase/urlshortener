'use client'

import { Suspense, useCallback, useRef, useState } from 'react'
import NextLink from 'next/link'
import { useParams, usePathname, useRouter, useSearchParams } from 'next/navigation'
import { EmptyState } from '@/components/common/EmptyState'
import { BuildTab } from '@/components/pages/BuildTab'
import { DesignTab } from '@/components/pages/DesignTab'
import { EditorTopBar, PAGES_HREF } from '@/components/pages/EditorTopBar'
import { PhonePreview } from '@/components/pages/PhonePreview'
import { TrackTab } from '@/components/pages/TrackTab'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ApiError } from '@/lib/api'
import { usePage } from '@/lib/hooks'
import type { Page, PageDesign } from '@/types'

const TABS = ['build', 'design', 'track'] as const
type EditorTab = (typeof TABS)[number]

const parseTab = (v: string | null): EditorTab => (TABS.includes(v as EditorTab) ? (v as EditorTab) : 'build')

/** JSON with object keys sorted: the server returns design keys alphabetically, the editor in preset order. */
const canonical = (v: unknown): string =>
  v != null && typeof v === 'object' && !Array.isArray(v)
    ? `{${Object.keys(v)
        .sort()
        .map((k) => `${JSON.stringify(k)}:${canonical((v as Record<string, unknown>)[k])}`)
        .join(',')}}`
    : JSON.stringify(v)

/** Designs are small plain JSON objects; key order does not matter. */
const sameDesign = (a: PageDesign, b: PageDesign) => canonical(a) === canonical(b)

function EditorSkeleton() {
  return (
    <div className="space-y-6 pt-8" aria-busy="true" aria-label="Loading page">
      <Skeleton className="h-5 w-20" />
      <Skeleton className="h-10 w-80" />
      <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_375px]">
        <div className="space-y-3">
          <Skeleton className="h-10 w-64" />
          <Skeleton className="h-16 rounded-xl" />
          <Skeleton className="h-16 rounded-xl" />
        </div>
        <Skeleton className="hidden h-[700px] rounded-[32px] lg:block" />
      </div>
    </div>
  )
}

function Editor({ page }: { page: Page }) {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const tab = parseTab(searchParams.get('tab'))
  // Unsaved Design-tab edits shown in the preview until the server page catches up.
  const [designOverride, setDesignOverride] = useState<PageDesign | null>(null)
  if (designOverride && sameDesign(designOverride, page.design)) setDesignOverride(null)
  const previewPage = designOverride ? { ...page, design: designOverride } : page

  // The Design tab registers its flush while mounted; publish sends pending edits first.
  const designFlush = useRef<(() => Promise<void>) | null>(null)
  const registerDesignFlush = useCallback((flush: (() => Promise<void>) | null) => {
    designFlush.current = flush
  }, [])
  const flushPendingSaves = useCallback(async () => {
    await designFlush.current?.()
  }, [])

  function setTab(next: string) {
    const sp = new URLSearchParams(searchParams.toString())
    if (next === 'build') sp.delete('tab')
    else sp.set('tab', next)
    const qs = sp.toString()
    router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false })
  }

  return (
    <div className="space-y-6 pb-12">
      <EditorTopBar page={page} flushPendingSaves={flushPendingSaves} designPending={designOverride != null} />
      <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_375px]">
        <Tabs value={tab} onValueChange={setTab} className="min-w-0">
          <TabsList>
            <TabsTrigger value="build">Build</TabsTrigger>
            <TabsTrigger value="design">Design</TabsTrigger>
            <TabsTrigger value="track">Track</TabsTrigger>
          </TabsList>
          <TabsContent value="build" className="pt-4">
            <BuildTab page={page} />
          </TabsContent>
          <TabsContent value="design" className="pt-4">
            <DesignTab page={page} onDesignChange={setDesignOverride} registerFlush={registerDesignFlush} />
          </TabsContent>
          <TabsContent value="track" className="pt-4">
            <TrackTab pageId={page.id} />
          </TabsContent>
        </Tabs>
        <aside className="sticky top-6 hidden self-start lg:block" aria-label="Page preview">
          <PhonePreview page={previewPage} />
        </aside>
      </div>
    </div>
  )
}

function EditorPageInner() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)
  const valid = Number.isInteger(id) && id > 0
  const { data: page, isLoading, isError, error, isFetching, refetch } = usePage(valid ? id : null)

  if (!valid || (error instanceof ApiError && error.status === 404)) {
    return (
      <div className="pt-8">
        <EmptyState
          title="Page not found"
          description="It may have been deleted, or it belongs to another group."
          action={
            <Button asChild>
              <NextLink href={PAGES_HREF}>Back to Pages</NextLink>
            </Button>
          }
        />
      </div>
    )
  }
  if (isLoading) return <EditorSkeleton />
  if (isError || !page) {
    return (
      <div className="space-y-4 pt-8">
        <NextLink href={PAGES_HREF} className="text-sm font-semibold text-primary hover:underline">
          ← Pages
        </NextLink>
        <EmptyState
          title="Couldn't load the page"
          description={error instanceof ApiError ? error.message : 'Something went wrong.'}
          action={
            <Button type="button" variant="secondary" disabled={isFetching} onClick={() => refetch()}>
              Retry
            </Button>
          }
        />
      </div>
    )
  }
  return <Editor page={page} />
}

/** Page editor (spec §5.7): top bar, Build | Design | Track tabs, and a live phone preview. */
export default function PageEditorPage() {
  return (
    // useSearchParams needs a Suspense boundary.
    <Suspense fallback={<EditorSkeleton />}>
      <EditorPageInner />
    </Suspense>
  )
}
