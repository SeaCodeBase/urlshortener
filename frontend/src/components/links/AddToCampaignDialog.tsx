'use client'

import { useId, useState, type FormEvent } from 'react'
import NextLink from 'next/link'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ApiError } from '@/lib/api'
import { useCampaign, useCampaignMutations, useCampaigns } from '@/lib/hooks'
import type { Link } from '@/types'

export interface AddToCampaignDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  link: Link
}

/** "Add to campaign": adds the link to a channel of one of the group's campaigns. */
export function AddToCampaignDialog({ open, onOpenChange, link }: AddToCampaignDialogProps) {
  const { addLinks } = useCampaignMutations()
  const saving = addLinks.isPending
  return (
    <Dialog open={open} onOpenChange={(o) => !saving && onOpenChange(o)}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">Add to campaign</DialogTitle>
          <DialogDescription>Track this link in a channel of one of your campaigns.</DialogDescription>
        </DialogHeader>
        {/* Mounted only while open so the draft resets each time. */}
        {open && <AddToCampaignBody link={link} onClose={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function AddToCampaignBody({ link, onClose }: { link: Link; onClose: () => void }) {
  const id = useId()
  const { data, isLoading } = useCampaigns()
  const { addLinks } = useCampaignMutations()
  const campaigns = data ?? []
  const [campaignId, setCampaignId] = useState('')
  const [channelId, setChannelId] = useState('')
  const [error, setError] = useState<string | null>(null)
  const campaign = campaigns.find((c) => String(c.id) === campaignId)
  const detail = useCampaign(campaign ? campaign.id : null)
  const channels = detail.data?.channels ?? []
  const noChannels = !!campaign && !!detail.data && channels.length === 0
  const channel = channels.find((c) => String(c.id) === channelId)
  const valid = !!campaign && !!channel

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid || !campaign || !channel || addLinks.isPending) return
    setError(null)
    try {
      await addLinks.mutateAsync({ campaignId: campaign.id, channelId: channel.id, linkIds: [link.id] })
      toast.success(`Added to ${campaign.name}`)
      onClose()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't add the link to the campaign")
    }
  }

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading campaigns…</p>

  if (campaigns.length === 0) {
    return (
      <>
        <div className="space-y-1 text-sm">
          <p className="text-muted-foreground">You don&apos;t have any campaigns yet</p>
          <NextLink href="/dashboard/campaigns" className="font-semibold text-primary hover:underline">
            Create campaign
          </NextLink>
        </div>
        <DialogFooter>
          <Button type="button" variant="secondary" onClick={onClose}>
            Cancel
          </Button>
        </DialogFooter>
      </>
    )
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor={`${id}-campaign`}>Campaign</Label>
        <Select
          value={campaignId}
          onValueChange={(v) => {
            setCampaignId(v)
            setChannelId('')
            setError(null)
          }}
        >
          <SelectTrigger id={`${id}-campaign`} aria-label="Campaign" className="w-full">
            <SelectValue placeholder="Choose a campaign" />
          </SelectTrigger>
          <SelectContent>
            {campaigns.map((c) => (
              <SelectItem key={c.id} value={String(c.id)}>
                {c.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${id}-channel`}>Channel</Label>
        <Select value={channelId} onValueChange={setChannelId} disabled={!campaign || channels.length === 0}>
          <SelectTrigger id={`${id}-channel`} aria-label="Channel" className="w-full">
            <SelectValue placeholder="Choose a channel" />
          </SelectTrigger>
          <SelectContent>
            {channels.map((c) => (
              <SelectItem key={c.id} value={String(c.id)}>
                {c.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {noChannels && <p className="text-sm text-muted-foreground">This campaign has no channels</p>}
      </div>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onClose} disabled={addLinks.isPending}>
          Cancel
        </Button>
        <Button type="submit" disabled={!valid || addLinks.isPending}>
          Add
        </Button>
      </DialogFooter>
    </form>
  )
}
