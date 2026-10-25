'use client'

import { useState } from 'react'
import NextLink from 'next/link'
import { MoreHorizontal, Pencil, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ApiError } from '@/lib/api'
import { formatDate, formatNumber } from '@/lib/format'
import { useCampaignMutations } from '@/lib/hooks'
import type { CampaignSummary } from '@/types'
import { CampaignDialog } from './CampaignDialog'

function plural(n: number, one: string, many: string): string {
  return `${formatNumber(n)} ${n === 1 ? one : many}`
}

/** "2 channels · 1 link · 1,240 engagements". */
export function campaignMeta(c: Pick<CampaignSummary, 'channel_count' | 'link_count' | 'engagements'>): string {
  return [
    plural(c.channel_count, 'channel', 'channels'),
    plural(c.link_count, 'link', 'links'),
    plural(c.engagements, 'engagement', 'engagements'),
  ].join(' · ')
}

/** Campaigns list card: name (link to detail), description, counts, created date and ⋯ Edit / Delete. */
export function CampaignCard({ campaign }: { campaign: CampaignSummary }) {
  const [editOpen, setEditOpen] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const { remove } = useCampaignMutations()
  const href = `/dashboard/campaigns/${campaign.id}`

  function onDelete() {
    return remove.mutateAsync(campaign.id).then(
      () => {
        toast.success('Campaign deleted')
        setConfirmDelete(false)
      },
      (err: unknown) => {
        toast.error(err instanceof ApiError ? err.message : "Couldn't delete the campaign")
        setConfirmDelete(false)
      },
    )
  }

  return (
    <article className="flex items-start gap-4 rounded-xl bg-card p-6 transition-shadow hover:shadow-sm">
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <NextLink
          href={href}
          className="min-w-0 truncate text-xl leading-7 font-semibold text-foreground hover:underline"
        >
          {campaign.name}
        </NextLink>
        {campaign.description && <p className="text-sm text-muted-foreground">{campaign.description}</p>}
        <div className="mt-3 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-muted-foreground">
          <span>{campaignMeta(campaign)}</span>
          <span>{formatDate(campaign.created_at)}</span>
        </div>
      </div>

      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="secondary" size="icon-sm" aria-label="More" className="size-8 shrink-0">
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-48">
          <DropdownMenuItem onSelect={() => setEditOpen(true)}>
            <Pencil />
            Edit
          </DropdownMenuItem>
          <DropdownMenuItem variant="destructive" onSelect={() => setConfirmDelete(true)}>
            <Trash2 />
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <CampaignDialog open={editOpen} onOpenChange={setEditOpen} campaign={campaign} />
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title={`Delete ${campaign.name}?`}
        description="Links stay in your account; only the campaign and its channel assignments are removed."
        confirmLabel="Delete"
        destructive
        loading={remove.isPending}
        onConfirm={onDelete}
      />
    </article>
  )
}
