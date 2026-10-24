'use client'

import { useState } from 'react'
import NextLink from 'next/link'
import { useRouter } from 'next/navigation'
import { ArrowLeft, Pencil, Share2 } from 'lucide-react'
import { Favicon } from '@/components/common/Favicon'
import { Button } from '@/components/ui/button'
import { formatShortUrl } from '@/lib/format'
import type { Link } from '@/types'
import { LinkActionsMenu } from './LinkActionsMenu'
import { ShareDialog } from './ShareDialog'

/** Details page title row: back, favicon, title; ⋯ menu, Edit, Share on the right (spec §5.4). */
export function LinkDetailsHeader({ link }: { link: Link }) {
  const router = useRouter()
  const [shareOpen, setShareOpen] = useState(false)
  const title = link.title || formatShortUrl(link.short_url)

  return (
    <div className="flex items-center justify-between gap-4 pt-8 pb-6">
      <div className="flex min-w-0 items-center gap-3">
        <NextLink
          href="/dashboard/links"
          aria-label="Back to links"
          className="inline-flex size-8 shrink-0 items-center justify-center rounded text-foreground hover:bg-accent"
        >
          <ArrowLeft className="size-5" />
        </NextLink>
        <Favicon url={link.original_url} src={link.favicon_url || undefined} size={40} />
        <h1 className="truncate text-[32px] leading-10 font-semibold tracking-[0.32px] text-foreground">{title}</h1>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <LinkActionsMenu link={link} triggerClassName="size-10" onDeleted={() => router.push('/dashboard/links')} />
        <Button variant="secondary" size="icon" aria-label="Edit" asChild>
          <NextLink href={`/dashboard/links/${link.id}/edit`}>
            <Pencil className="size-5" />
          </NextLink>
        </Button>
        <Button type="button" variant="outline" onClick={() => setShareOpen(true)}>
          <Share2 className="size-5" />
          Share
        </Button>
      </div>
      <ShareDialog open={shareOpen} onOpenChange={setShareOpen} shortUrl={link.short_url} title={link.title} />
    </div>
  )
}
