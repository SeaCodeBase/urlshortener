'use client'

import { useId, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { ApiError } from '@/lib/api'
import { useCampaign, useCampaignMutations } from '@/lib/hooks'
import type { CampaignDetail, CampaignInput, CampaignSummary } from '@/types'
import { ChannelPicker } from './ChannelPicker'

const MAX_NAME = 120
const MAX_DESCRIPTION = 500

export interface CampaignDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Edit mode when given. A summary (no channels) is completed with `useCampaign(id)`. */
  campaign?: CampaignDetail | CampaignSummary
}

function hasChannels(c: CampaignDetail | CampaignSummary): c is CampaignDetail {
  return Array.isArray((c as CampaignDetail).channels)
}

/** Create or edit a campaign: Name, Description, Channels, "Auto-add UTM parameters". */
export function CampaignDialog({ open, onOpenChange, campaign }: CampaignDialogProps) {
  const editing = campaign != null
  const [saving, setSaving] = useState(false)
  // A summary has no channel list: fetch the detail while the dialog is open.
  const needsDetail = open && campaign != null && !hasChannels(campaign)
  const detailQuery = useCampaign(needsDetail ? campaign.id : null)
  const detail = campaign && hasChannels(campaign) ? campaign : detailQuery.data

  return (
    <Dialog open={open} onOpenChange={(o) => !saving && onOpenChange(o)}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">{editing ? 'Edit campaign' : 'Create campaign'}</DialogTitle>
          <DialogDescription className="sr-only">
            {editing ? 'Edit campaign' : 'Create campaign'}
          </DialogDescription>
        </DialogHeader>
        {/* Mounted only while open (and once the channels are known) so the draft resets each time. */}
        {open &&
          (editing && !detail ? (
            <div className="flex flex-col gap-3" aria-busy="true" aria-label="Loading campaign">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-20 w-full" />
              <Skeleton className="h-5 w-1/3" />
            </div>
          ) : (
            <CampaignForm
              key={campaign?.id ?? 'new'}
              campaign={detail}
              onCancel={() => onOpenChange(false)}
              onSavingChange={setSaving}
              onSaved={() => onOpenChange(false)}
            />
          ))}
      </DialogContent>
    </Dialog>
  )
}

function CampaignForm({
  campaign,
  onCancel,
  onSavingChange,
  onSaved,
}: {
  campaign?: CampaignDetail
  onCancel: () => void
  onSavingChange: (saving: boolean) => void
  onSaved: () => void
}) {
  const ids = { name: useId(), description: useId(), utm: useId(), utmHelp: useId() }
  const { create, update } = useCampaignMutations()
  const [name, setName] = useState(campaign?.name ?? '')
  const [description, setDescription] = useState(campaign?.description ?? '')
  const [autoUtm, setAutoUtm] = useState(campaign?.auto_utm ?? false)
  const [channelIds, setChannelIds] = useState<number[]>(campaign?.channels.map((c) => c.id) ?? [])
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const trimmed = name.trim()
  const valid = trimmed.length > 0 && trimmed.length <= MAX_NAME

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid || saving) return
    // PUT replaces the channel set, so the full selection is always sent.
    const input: CampaignInput = {
      name: trimmed,
      description: description.trim(),
      auto_utm: autoUtm,
      channel_ids: channelIds,
      new_channels: [],
    }
    setError(null)
    setSaving(true)
    onSavingChange(true)
    try {
      if (campaign) {
        await update.mutateAsync({ id: campaign.id, input })
        toast.success('Campaign updated')
      } else {
        await create.mutateAsync(input)
        toast.success('Campaign created')
      }
      setSaving(false)
      onSavingChange(false)
      onSaved()
    } catch (err) {
      setError(
        err instanceof ApiError || err instanceof Error
          ? err.message
          : campaign
            ? "Couldn't update the campaign"
            : "Couldn't create the campaign",
      )
      setSaving(false)
      onSavingChange(false)
    }
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <Label htmlFor={ids.name}>Name</Label>
        <Input
          id={ids.name}
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={MAX_NAME}
          required
          autoFocus
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={ids.description}>Description</Label>
        <Textarea
          id={ids.description}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          maxLength={MAX_DESCRIPTION}
          rows={3}
        />
      </div>
      <fieldset className="flex flex-col gap-2">
        <legend className="mb-2 text-sm leading-none font-medium">Channels</legend>
        <ChannelPicker value={channelIds} onChange={setChannelIds} />
      </fieldset>
      <div className="flex items-start gap-3">
        <Checkbox
          id={ids.utm}
          aria-label="Auto-add UTM parameters"
          checked={autoUtm}
          onCheckedChange={setAutoUtm}
          className="mt-0.5"
        />
        <div className="flex flex-col gap-1">
          <Label htmlFor={ids.utm}>Auto-add UTM parameters</Label>
          <p id={ids.utmHelp} className="text-sm text-muted-foreground">
            Adds utm_medium=bitly, utm_source={'{channel}'} and utm_campaign={'{name}'} to links created from a channel
          </p>
        </div>
      </div>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onCancel} disabled={saving}>
          Cancel
        </Button>
        <Button type="submit" disabled={saving || !valid}>
          {campaign ? 'Save' : 'Create campaign'}
        </Button>
      </DialogFooter>
    </form>
  )
}
