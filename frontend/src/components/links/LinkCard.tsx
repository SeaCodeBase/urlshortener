'use client'

import { useState } from 'react'
import NextLink from 'next/link'
import { BarChart2, Calendar, CornerDownRight, Pencil, Share2 } from 'lucide-react'
import { CopyButton } from '@/components/common/CopyButton'
import { Favicon } from '@/components/common/Favicon'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { formatDate, formatNumber, formatShortUrl } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { Link } from '@/types'
import { LinkActionsMenu } from './LinkActionsMenu'
import { ShareDialog } from './ShareDialog'
import { TagEditor } from './TagEditor'

/** "1 engagement" / "3 engagements". */
export function engagementsLabel(n: number): string {
  return `${formatNumber(n)} ${n === 1 ? 'engagement' : 'engagements'}`
}

export interface LinkCardProps {
  link: Link
  selected: boolean
  onSelectedChange: (selected: boolean) => void
  /** Grid view stacks the action icons above the content. */
  layout?: 'default' | 'grid'
}

/** Icon buttons on the right of a card: Edit, Share, View details, More. */
export function LinkCardActions({ link, className }: { link: Link; className?: string }) {
  const [shareOpen, setShareOpen] = useState(false)
  return (
    <div className={cn('flex shrink-0 items-center gap-2', className)}>
      <Button asChild variant="secondary" size="icon-sm" className="size-8">
        <NextLink href={`/dashboard/links/${link.id}/edit`} aria-label="Edit link">
          <Pencil className="size-4" />
        </NextLink>
      </Button>
      <Button type="button" variant="secondary" size="icon-sm" className="size-8" aria-label="Share" onClick={() => setShareOpen(true)}>
        <Share2 className="size-4" />
      </Button>
      <Button asChild variant="secondary" size="icon-sm" className="size-8">
        <NextLink href={`/dashboard/links/${link.id}`} aria-label="View details">
          <BarChart2 className="size-4" />
        </NextLink>
      </Button>
      <LinkActionsMenu link={link} />
      <ShareDialog open={shareOpen} onOpenChange={setShareOpen} shortUrl={link.short_url} title={link.title} />
    </div>
  )
}

/** Default-view link card (spec §5.2 / §3: 12px radius, 16/16/24/32 padding, no border). */
export function LinkCard({ link, selected, onSelectedChange, layout = 'default' }: LinkCardProps) {
  const title = link.title || formatShortUrl(link.short_url)
  const grid = layout === 'grid'
  return (
    <article
      className={cn(
        'flex gap-4 rounded-xl bg-card pt-4 pr-4 pb-6 pl-8 transition-shadow hover:shadow-sm',
        selected && 'ring-2 ring-primary/40',
        grid && 'h-full flex-col',
      )}
    >
      <div className={cn('flex gap-4', grid ? 'items-center justify-between' : 'contents')}>
        <div className="flex shrink-0 items-start gap-4 pt-1.5">
          <Checkbox
            checked={selected}
            onCheckedChange={onSelectedChange}
            className="mt-1"
            aria-label={`Select ${title}`}
          />
          <Favicon url={link.original_url} src={link.favicon_url || undefined} />
        </div>
        {grid && <LinkCardActions link={link} />}
      </div>

      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex items-start gap-2">
          <NextLink
            href={`/dashboard/links/${link.id}`}
            className="min-w-0 truncate text-xl leading-7 font-semibold text-foreground hover:underline"
          >
            {title}
          </NextLink>
          {link.archived && (
            <Badge variant="secondary" className="mt-1">
              Hidden
            </Badge>
          )}
        </div>
        <div className="flex min-w-0 items-center gap-1">
          <a
            href={link.short_url}
            target="_blank"
            rel="noopener noreferrer"
            className="truncate text-base font-medium text-primary hover:underline"
          >
            {formatShortUrl(link.short_url)}
          </a>
          <CopyButton text={link.short_url} />
        </div>
        <div className="flex min-w-0 items-center gap-1 text-sm text-foreground">
          <CornerDownRight className="size-4 shrink-0 text-muted-foreground" aria-hidden />
          <span className="truncate" title={link.original_url}>
            {link.original_url}
          </span>
        </div>
        <div className="mt-4 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-muted-foreground">
          <span className="inline-flex items-center gap-1.5">
            <BarChart2 className="size-4" aria-hidden />
            {engagementsLabel(link.total_clicks)}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <Calendar className="size-4" aria-hidden />
            {formatDate(link.created_at)}
          </span>
          <TagEditor linkId={link.id} tags={link.tags} />
        </div>
      </div>

      {!grid && <LinkCardActions link={link} className="self-start" />}
    </article>
  )
}
