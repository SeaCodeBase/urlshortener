'use client'

import { Favicon } from '@/components/common/Favicon'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatDate, formatNumber, formatShortUrl } from '@/lib/format'
import type { CampaignDetail } from '@/types'
import { linkLabel } from './AddLinkToChannelDialog'

/** Every link membership of a campaign (one row per channel a link is in): Link · Channel · Engagements · Created. */
export function CampaignLinksTable({ campaign }: { campaign: CampaignDetail }) {
  const rows = campaign.channels.flatMap((c) => c.links.map((l) => ({ ...l, channel_id: c.id, channel_name: c.name })))

  if (rows.length === 0) {
    return <p className="rounded-xl bg-card p-6 text-sm text-muted-foreground">No links in this campaign yet.</p>
  }

  return (
    <div className="rounded-xl bg-card p-6">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Link</TableHead>
            <TableHead>Channel</TableHead>
            <TableHead className="text-right">Engagements</TableHead>
            <TableHead>Created</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((l) => (
            <TableRow key={`${l.channel_id}-${l.id}`}>
              <TableCell>
                <div className="flex min-w-0 items-center gap-3">
                  <Favicon url={l.original_url} src={l.favicon_url} size={24} />
                  <div className="min-w-0">
                    <p className="truncate font-semibold text-foreground">{linkLabel(l)}</p>
                    <p className="truncate text-primary">{formatShortUrl(l.short_url)}</p>
                  </div>
                </div>
              </TableCell>
              <TableCell>{l.channel_name}</TableCell>
              <TableCell className="text-right">{formatNumber(l.total_clicks)}</TableCell>
              <TableCell>{formatDate(l.created_at)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
