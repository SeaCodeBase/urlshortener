'use client'

import Link from 'next/link'
import { ArrowRight } from 'lucide-react'
import { CopyButton } from '@/components/common/CopyButton'
import { Favicon } from '@/components/common/Favicon'
import { formatShortUrl } from '@/lib/format'
import { useLinks } from '@/lib/hooks'

/** Last 5 links of the active group. */
export function RecentLinks() {
  const { data, isLoading } = useLinks({ limit: 5 })
  const links = data?.pages[0]?.links.slice(0, 5) ?? []

  return (
    <div className="rounded-xl border border-border bg-card p-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-foreground">Recent links</h2>
        <Link href="/dashboard/links" className="inline-flex items-center gap-1 text-sm font-semibold text-primary hover:underline">
          View all
          <ArrowRight className="size-4" />
        </Link>
      </div>
      {isLoading ? (
        <p className="mt-4 text-sm text-muted-foreground">Loading…</p>
      ) : links.length === 0 ? (
        <p className="mt-4 text-sm text-muted-foreground">
          No links yet.{' '}
          <Link href="/dashboard/links/create" className="font-semibold text-primary hover:underline">
            Create your first link
          </Link>
        </p>
      ) : (
        <ul className="mt-2 divide-y divide-border">
          {links.map((l) => (
            <li key={l.id} className="flex items-center gap-3 py-3">
              <Favicon url={l.original_url} src={l.favicon_url} />
              <div className="min-w-0 flex-1">
                <div className="truncate font-semibold text-foreground">{l.title || l.original_url}</div>
                <div className="flex items-center gap-1">
                  <a
                    href={l.short_url}
                    target="_blank"
                    rel="noreferrer"
                    className="truncate text-sm text-primary hover:underline"
                  >
                    {formatShortUrl(l.short_url)}
                  </a>
                  <CopyButton text={l.short_url} />
                </div>
              </div>
              <Link href={`/dashboard/links/${l.id}`} className="shrink-0 text-sm font-semibold text-primary hover:underline">
                View details
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
