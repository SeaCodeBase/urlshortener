'use client'

import { useState } from 'react'
import NextLink from 'next/link'
import { BarChart2, Pencil, Share2 } from 'lucide-react'
import { CopyButton } from '@/components/common/CopyButton'
import { ShareDialog } from '@/components/links/ShareDialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { formatNumber, formatShortUrl } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { Page } from '@/types'
import { PageActionsMenu } from './PageActionsMenu'

/** "1 view" / "12 views". */
export function viewsLabel(n: number): string {
  return `${formatNumber(n)} ${n === 1 ? 'view' : 'views'}`
}

function PageAvatar({ page }: { page: Page }) {
  if (page.avatar_url) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img src={page.avatar_url} alt="" className="size-14 shrink-0 rounded-full object-cover" />
    )
  }
  const initial = (page.title || page.slug).trim().charAt(0).toUpperCase()
  return (
    <div
      aria-hidden
      className="flex size-14 shrink-0 items-center justify-center rounded-full text-xl font-bold"
      style={{ background: page.design.button.color, color: page.design.button.text_color }}
    >
      {initial}
    </div>
  )
}

/** Draft / Published chip, plus an amber dot when a published page has unpublished changes. */
export function PageStatusChip({ page }: { page: Page }) {
  const published = page.status === 'published'
  return (
    <span className="inline-flex items-center gap-1.5">
      <span
        className={cn(
          'rounded-full px-2.5 py-0.5 text-xs font-medium',
          published ? 'bg-success/10 text-success' : 'bg-muted text-muted-foreground',
        )}
      >
        {published ? 'Published' : 'Draft'}
      </span>
      {published && page.has_unpublished_changes && (
        <Tooltip>
          <TooltipTrigger asChild>
            <span
              role="img"
              tabIndex={0}
              aria-label="Unpublished changes"
              className="inline-block size-2 rounded-full bg-amber-500"
            />
          </TooltipTrigger>
          <TooltipContent>Unpublished changes</TooltipContent>
        </Tooltip>
      )}
    </span>
  )
}

/** Pages list card: avatar, title, URL + copy, views, status, Edit / Share / ⋯. */
export function PageCard({ page }: { page: Page }) {
  const [shareOpen, setShareOpen] = useState(false)
  const editHref = `/dashboard/pages/${page.id}`
  const title = page.title || page.slug

  return (
    <article className="flex items-start gap-4 rounded-xl bg-card p-6 transition-shadow hover:shadow-sm">
      <NextLink href={editHref} tabIndex={-1} aria-hidden>
        <PageAvatar page={page} />
      </NextLink>

      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex items-center gap-2">
          <NextLink
            href={editHref}
            className="min-w-0 truncate text-xl leading-7 font-semibold text-foreground hover:underline"
          >
            {title}
          </NextLink>
          {page.archived && <Badge variant="secondary">Hidden</Badge>}
        </div>
        <span className="inline-flex min-w-0 items-center gap-1 text-sm">
          <a
            href={page.url}
            target="_blank"
            rel="noopener noreferrer"
            className="truncate font-medium text-primary hover:underline"
          >
            {formatShortUrl(page.url)}
          </a>
          <CopyButton text={page.url} />
        </span>
        <div className="mt-3 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-muted-foreground">
          <span className="inline-flex items-center gap-1.5">
            <BarChart2 className="size-4" aria-hidden />
            {viewsLabel(page.views)}
          </span>
          <PageStatusChip page={page} />
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-2">
        <Button asChild variant="secondary" size="sm">
          <NextLink href={editHref}>
            <Pencil />
            Edit
          </NextLink>
        </Button>
        <Button type="button" variant="secondary" size="sm" onClick={() => setShareOpen(true)}>
          <Share2 />
          Share
        </Button>
        <PageActionsMenu page={page} />
      </div>

      <ShareDialog open={shareOpen} onOpenChange={setShareOpen} shortUrl={page.url} title={title} />
    </article>
  )
}
