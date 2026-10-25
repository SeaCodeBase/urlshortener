'use client'

import { useId, useState } from 'react'
import { MoreHorizontal, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
import { CopyButton } from '@/components/common/CopyButton'
import { Favicon } from '@/components/common/Favicon'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ApiError } from '@/lib/api'
import { formatNumber, formatShortUrl } from '@/lib/format'
import { useCampaignMutations } from '@/lib/hooks'
import type { CampaignDetail, CampaignLinkItem, ChannelDetail, ChannelStats } from '@/types'
import { AddLinkToChannelDialog, linkLabel } from './AddLinkToChannelDialog'

function plural(n: number, one: string, many: string): string {
  return `${formatNumber(n)} ${n === 1 ? one : many}`
}

/** "2 links · 45 engagements" from the stats of the current preset, "—" while they load. */
export function channelMeta(stats: Pick<ChannelStats, 'links' | 'engagements'> | undefined): string {
  if (!stats) return '—'
  return `${plural(stats.links, 'link', 'links')} · ${plural(stats.engagements, 'engagement', 'engagements')}`
}

/** A campaign channel: header with counts, its links (short URL, copy, clicks, ⋯ Remove) and "Add link to channel". */
export function ChannelCard({
  campaign,
  channel,
  stats,
}: {
  campaign: CampaignDetail
  channel: ChannelDetail
  /** This channel's entry of the campaign stats for the current preset; undefined while loading. */
  stats?: ChannelStats
}) {
  const headingId = useId()
  const [addOpen, setAddOpen] = useState(false)
  const [removing, setRemoving] = useState<CampaignLinkItem | null>(null)
  const { removeLink } = useCampaignMutations()

  function onRemove() {
    if (!removing) return
    return removeLink.mutateAsync({ campaignId: campaign.id, channelId: channel.id, linkId: removing.id }).then(
      () => {
        toast.success(`Removed from ${channel.name}`)
        setRemoving(null)
      },
      (err: unknown) => {
        toast.error(err instanceof ApiError ? err.message : "Couldn't remove the link")
        setRemoving(null)
      },
    )
  }

  return (
    <section aria-labelledby={headingId} className="rounded-xl bg-card p-6">
      <header className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 id={headingId} className="text-lg font-semibold text-foreground">
          {channel.name}
        </h3>
        <span className="text-sm text-muted-foreground">{channelMeta(stats)}</span>
      </header>

      {channel.links.length > 0 && (
        <ul className="mt-4 divide-y divide-border">
          {channel.links.map((l) => {
            const label = linkLabel(l)
            return (
              <li key={l.id} className="flex items-center gap-3 py-3">
                <Favicon url={l.original_url} src={l.favicon_url} size={32} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-semibold text-foreground">{label}</p>
                  <div className="flex min-w-0 items-center gap-1">
                    <a
                      href={l.short_url}
                      target="_blank"
                      rel="noreferrer"
                      className="truncate text-sm text-primary hover:underline"
                    >
                      {formatShortUrl(l.short_url)}
                    </a>
                    <CopyButton text={l.short_url} size="icon" className="size-7" />
                  </div>
                </div>
                <span className="shrink-0 text-sm text-muted-foreground">
                  {plural(l.total_clicks, 'engagement', 'engagements')}
                </span>
                <DropdownMenu modal={false}>
                  <DropdownMenuTrigger asChild>
                    <Button
                      type="button"
                      variant="secondary"
                      size="icon-sm"
                      aria-label={`More actions for ${label}`}
                      className="size-8 shrink-0"
                    >
                      <MoreHorizontal className="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-52">
                    <DropdownMenuItem variant="destructive" onSelect={() => setRemoving(l)}>
                      <Trash2 />
                      Remove from channel
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </li>
            )
          })}
        </ul>
      )}

      <Button type="button" variant="secondary" className="mt-4" onClick={() => setAddOpen(true)}>
        <Plus className="size-4" />
        Add link to channel
      </Button>

      <AddLinkToChannelDialog open={addOpen} onOpenChange={setAddOpen} campaign={campaign} channel={channel} />
      <ConfirmDialog
        open={removing != null}
        onOpenChange={(o) => !o && setRemoving(null)}
        title="Remove from channel?"
        description="The link itself is not deleted."
        confirmLabel="Remove"
        destructive
        loading={removeLink.isPending}
        onConfirm={() => onRemove()}
      />
    </section>
  )
}
