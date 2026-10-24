'use client'

import NextLink from 'next/link'
import { BarChart2, Calendar } from 'lucide-react'
import { CopyButton } from '@/components/common/CopyButton'
import { Favicon } from '@/components/common/Favicon'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { formatDate, formatShortUrl } from '@/lib/format'
import { cn } from '@/lib/utils'
import { engagementsLabel, LinkCardActions, type LinkCardProps } from './LinkCard'

/** Compact view: one row per link; actions appear on hover/focus. */
export function LinkCardCompact({ link, selected, onSelectedChange }: Omit<LinkCardProps, 'layout'>) {
  const title = link.title || formatShortUrl(link.short_url)
  return (
    <article
      className={cn(
        'group flex min-h-14 items-center gap-4 rounded-xl bg-card py-2 pr-4 pl-8',
        selected && 'ring-2 ring-primary/40',
      )}
    >
      <Checkbox checked={selected} onCheckedChange={onSelectedChange} aria-label={`Select ${title}`} />
      <Favicon url={link.original_url} src={link.favicon_url || undefined} size={24} />
      <NextLink
        href={`/dashboard/links/${link.id}`}
        className="min-w-0 max-w-[30%] truncate text-base font-semibold text-foreground hover:underline"
      >
        {title}
      </NextLink>
      {link.archived && <Badge variant="secondary">Hidden</Badge>}
      <div className="flex min-w-0 flex-1 items-center gap-1">
        <a
          href={link.short_url}
          target="_blank"
          rel="noopener noreferrer"
          className="truncate text-sm font-medium text-primary hover:underline"
        >
          {formatShortUrl(link.short_url)}
        </a>
        <CopyButton text={link.short_url} />
      </div>
      <span className="hidden shrink-0 items-center gap-1.5 text-sm text-muted-foreground md:inline-flex">
        <BarChart2 className="size-4" aria-hidden />
        {engagementsLabel(link.total_clicks)}
      </span>
      <span className="hidden shrink-0 items-center gap-1.5 text-sm text-muted-foreground lg:inline-flex">
        <Calendar className="size-4" aria-hidden />
        {formatDate(link.created_at)}
      </span>
      <LinkCardActions
        link={link}
        className="opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100"
      />
    </article>
  )
}
