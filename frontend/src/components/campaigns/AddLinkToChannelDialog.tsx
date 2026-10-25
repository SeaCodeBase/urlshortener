'use client'

import { useId, useMemo, useState, type FormEvent } from 'react'
import { Info } from 'lucide-react'
import { toast } from 'sonner'
import { Favicon } from '@/components/common/Favicon'
import { validateBackHalf } from '@/components/links/ShortLinkField'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ApiError } from '@/lib/api'
import { formatShortUrl } from '@/lib/format'
import { useCampaignMutations, useDebouncedValue, useLinks } from '@/lib/hooks'
import type { CampaignDetail, ChannelDetail, CreateLinkInChannelInput, Link } from '@/types'

export interface AddLinkToChannelDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  campaign: CampaignDetail
  channel: ChannelDetail
}

/** A link's display name: its title, else its destination without the protocol. */
export function linkLabel(link: Pick<Link, 'title' | 'original_url'>): string {
  return link.title?.trim() || formatShortUrl(link.original_url)
}

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof ApiError || err instanceof Error ? err.message : fallback
}

/** "Add link to channel": pick existing links of the group, or create a new one directly in the channel. */
export function AddLinkToChannelDialog({ open, onOpenChange, campaign, channel }: AddLinkToChannelDialogProps) {
  const [saving, setSaving] = useState(false)
  return (
    <Dialog open={open} onOpenChange={(o) => !saving && onOpenChange(o)}>
      <DialogContent className="sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">Add link to channel</DialogTitle>
          <DialogDescription>Add links to {channel.name} in {campaign.name}.</DialogDescription>
        </DialogHeader>
        {/* Mounted only while open so the draft resets each time. */}
        {open && (
          <Tabs defaultValue="pick" className="min-w-0">
            <TabsList>
              <TabsTrigger value="pick">Pick existing</TabsTrigger>
              <TabsTrigger value="create">Create link</TabsTrigger>
            </TabsList>
            <TabsContent value="pick" className="pt-4">
              <PickExisting
                campaign={campaign}
                channel={channel}
                onSavingChange={setSaving}
                onClose={() => onOpenChange(false)}
              />
            </TabsContent>
            <TabsContent value="create" className="pt-4">
              <CreateInChannel
                campaign={campaign}
                channel={channel}
                onSavingChange={setSaving}
                onClose={() => onOpenChange(false)}
              />
            </TabsContent>
          </Tabs>
        )}
      </DialogContent>
    </Dialog>
  )
}

interface BodyProps {
  campaign: CampaignDetail
  channel: ChannelDetail
  onSavingChange: (saving: boolean) => void
  onClose: () => void
}

function PickExisting({ campaign, channel, onSavingChange, onClose }: BodyProps) {
  const { addLinks } = useCampaignMutations()
  const [search, setSearch] = useState('')
  const q = useDebouncedValue(search.trim(), 300)
  const { data, isLoading, isError } = useLinks({ q, limit: 20 })
  const [selected, setSelected] = useState<number[]>([])
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const inChannel = useMemo(() => new Set(channel.links.map((l) => l.id)), [channel.links])
  const links = (data?.pages.flatMap((p) => p.links) ?? []).filter((l) => l.source === 'link' && !inChannel.has(l.id))
  const n = selected.length

  function toggle(id: number, on: boolean) {
    setSelected((prev) => (on ? [...prev, id] : prev.filter((x) => x !== id)))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (n === 0 || saving) return
    setError(null)
    setSaving(true)
    onSavingChange(true)
    try {
      await addLinks.mutateAsync({ campaignId: campaign.id, channelId: channel.id, linkIds: selected })
      toast.success(`Added to ${channel.name}`)
      setSaving(false)
      onSavingChange(false)
      onClose()
    } catch (err) {
      setError(errorMessage(err, "Couldn't add the links"))
      setSaving(false)
      onSavingChange(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-4">
      <Input
        type="search"
        aria-label="Search links"
        placeholder="Search links"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <div className="max-h-72 overflow-y-auto rounded-lg border border-border">
        {isLoading ? (
          <p className="p-4 text-sm text-muted-foreground">Loading links…</p>
        ) : isError ? (
          <p className="p-4 text-sm text-destructive">Failed to load links</p>
        ) : links.length === 0 ? (
          <p className="p-4 text-sm text-muted-foreground">No links to add.</p>
        ) : (
          <ul className="divide-y divide-border">
            {links.map((l) => {
              const label = linkLabel(l)
              const checked = selected.includes(l.id)
              return (
                <li key={l.id} className="flex items-center gap-3 px-4 py-3">
                  <Checkbox checked={checked} onCheckedChange={(on) => toggle(l.id, on)} aria-label={label} />
                  <Favicon url={l.original_url} src={l.favicon_url} size={24} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-foreground">{label}</p>
                    <p className="truncate text-sm text-primary">{formatShortUrl(l.short_url)}</p>
                  </div>
                </li>
              )
            })}
          </ul>
        )}
      </div>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onClose} disabled={saving}>
          Cancel
        </Button>
        <Button type="submit" disabled={n === 0 || saving}>
          {n === 0 ? 'Add links' : n === 1 ? 'Add 1 link' : `Add ${n} links`}
        </Button>
      </DialogFooter>
    </form>
  )
}

function isHttpUrl(value: string): boolean {
  try {
    const u = new URL(value)
    return u.protocol === 'http:' || u.protocol === 'https:'
  } catch {
    return false
  }
}

function CreateInChannel({ campaign, channel, onSavingChange, onClose }: BodyProps) {
  const id = useId()
  const { createLinkInChannel } = useCampaignMutations()
  const [url, setUrl] = useState('')
  const [title, setTitle] = useState('')
  const [backHalf, setBackHalf] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const trimmedUrl = url.trim()
  const backHalfError = validateBackHalf(backHalf.trim())
  const valid = isHttpUrl(trimmedUrl) && !backHalfError

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid || saving) return
    const input: CreateLinkInChannelInput = { original_url: trimmedUrl }
    if (title.trim()) input.title = title.trim()
    if (backHalf.trim()) input.custom_code = backHalf.trim()
    setError(null)
    setSaving(true)
    onSavingChange(true)
    try {
      await createLinkInChannel.mutateAsync({ campaignId: campaign.id, channelId: channel.id, input })
      toast.success('Link created')
      setSaving(false)
      onSavingChange(false)
      onClose()
    } catch (err) {
      setError(errorMessage(err, "Couldn't create the link"))
      setSaving(false)
      onSavingChange(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor={`${id}-url`}>Destination URL</Label>
        <Input
          id={`${id}-url`}
          type="url"
          required
          placeholder="https://example.com/my-long-url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${id}-title`}>Title (optional)</Label>
        <Input id={`${id}-title`} value={title} maxLength={255} onChange={(e) => setTitle(e.target.value)} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${id}-backhalf`}>Back-half (optional)</Label>
        <Input
          id={`${id}-backhalf`}
          value={backHalf}
          maxLength={255}
          placeholder="my-custom-back-half"
          aria-invalid={backHalfError ? true : undefined}
          aria-describedby={backHalfError ? `${id}-backhalf-error` : undefined}
          onChange={(e) => setBackHalf(e.target.value)}
        />
        {backHalfError && (
          <p id={`${id}-backhalf-error`} className="text-sm text-destructive">
            {backHalfError}
          </p>
        )}
      </div>
      {campaign.auto_utm && (
        <p className="flex items-start gap-2 rounded-lg bg-muted px-3 py-2 text-sm text-muted-foreground">
          <Info className="mt-0.5 size-4 shrink-0" aria-hidden />
          <span>
            {`UTM parameters will be added: utm_medium=bitly, utm_source=${channel.name}, utm_campaign=${campaign.name}`}
          </span>
        </p>
      )}
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onClose} disabled={saving}>
          Cancel
        </Button>
        <Button type="submit" disabled={!valid || saving}>
          Create link
        </Button>
      </DialogFooter>
    </form>
  )
}
