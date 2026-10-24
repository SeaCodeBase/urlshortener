'use client'

import { Suspense, useEffect, useRef, useState } from 'react'
import NextLink from 'next/link'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { Search } from 'lucide-react'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { PageCard } from '@/components/pages/PageCard'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { usePages } from '@/lib/hooks'

const NEW_PAGE_HREF = '/dashboard/pages/new'

function CardSkeleton() {
  return (
    <div className="flex items-start gap-4 rounded-xl bg-card p-6" aria-hidden>
      <Skeleton className="size-14 rounded-full" />
      <div className="flex flex-1 flex-col gap-2">
        <Skeleton className="h-6 w-1/3" />
        <Skeleton className="h-4 w-1/4" />
        <Skeleton className="mt-2 h-4 w-1/5" />
      </div>
    </div>
  )
}

function PagesList() {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const q = searchParams.get('q') ?? ''
  const hidden = searchParams.get('archived') === 'on'
  const query = usePages({ q: q || undefined, archived: hidden ? 'on' : undefined })
  const pages = query.data?.pages ?? []

  function setParams(patch: { q?: string; archived?: boolean }) {
    const next = new URLSearchParams(searchParams.toString())
    if (patch.q !== undefined) {
      if (patch.q) next.set('q', patch.q)
      else next.delete('q')
    }
    if (patch.archived !== undefined) {
      if (patch.archived) next.set('archived', 'on')
      else next.delete('archived')
    }
    const qs = next.toString()
    router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false })
  }

  // Debounced search box; the API does the filtering.
  const [text, setText] = useState(q)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current)
  }, [])
  function onSearch(value: string) {
    setText(value)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => setParams({ q: value.trim() }), 300)
  }

  let body
  if (query.isPending) {
    body = (
      <div className="flex flex-col gap-4" aria-busy="true" aria-label="Loading pages">
        <CardSkeleton />
        <CardSkeleton />
        <CardSkeleton />
      </div>
    )
  } else if (query.isError) {
    body = (
      <EmptyState
        title="Couldn't load pages"
        description="Something went wrong. Please try again."
        action={
          <Button type="button" variant="secondary" onClick={() => void query.refetch()}>
            Retry
          </Button>
        }
      />
    )
  } else if (pages.length === 0 && !q && !hidden) {
    body = (
      <EmptyState
        title="Pages that turn visits into engagements"
        description="Build a link-in-bio page with your links, text, images and social icons, then share one URL everywhere."
        action={
          <Button asChild>
            <NextLink href={NEW_PAGE_HREF}>Create your first page</NextLink>
          </Button>
        }
      />
    )
  } else if (pages.length === 0) {
    body = (
      <EmptyState
        title="No results found"
        description={
          <>
            Try adjusting your search, or{' '}
            <button
              type="button"
              className="font-medium text-primary hover:underline"
              onClick={() => setParams({ archived: !hidden })}
            >
              {hidden ? 'try searching active pages instead.' : 'try searching hidden pages instead.'}
            </button>
          </>
        }
      />
    )
  } else {
    body = (
      <div className="flex flex-col gap-4">
        {pages.map((p) => (
          <PageCard key={p.id} page={p} />
        ))}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4 pb-12">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="relative w-full sm:w-[300px]">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="search"
            value={text}
            onChange={(e) => onSearch(e.target.value)}
            placeholder="Search pages"
            aria-label="Search pages"
            className="bg-white pl-9 dark:bg-card"
          />
        </div>
        <Select value={hidden ? 'hidden' : 'active'} onValueChange={(v) => setParams({ archived: v === 'hidden' })}>
          <SelectTrigger aria-label="Show" className="w-auto min-w-36 bg-white dark:bg-card">
            <span className="text-muted-foreground">{'Show: '}</span>
            <SelectValue />
          </SelectTrigger>
          <SelectContent align="end">
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="hidden">Hidden</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {body}
    </div>
  )
}

export default function PagesPage() {
  return (
    <div>
      <PageHeader
        title="Pages"
        actions={
          <Button asChild>
            <NextLink href={NEW_PAGE_HREF}>Create page</NextLink>
          </Button>
        }
      />
      {/* useSearchParams needs a Suspense boundary for static rendering. */}
      <Suspense>
        <PagesList />
      </Suspense>
    </div>
  )
}
