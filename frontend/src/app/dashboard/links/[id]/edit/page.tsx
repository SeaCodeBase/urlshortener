'use client'

import { useId, useState, type FormEvent, type ReactNode } from 'react'
import NextLink from 'next/link'
import { useParams, useRouter } from 'next/navigation'
import { CornerDownRight, Info, Pencil, X } from 'lucide-react'
import { toast } from 'sonner'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { TagPicker } from '@/components/common/TagPicker'
import { ExpirationField, minExpiration, toDatetimeLocal } from '@/components/links/ExpirationField'
import { ShortLinkField, validateBackHalf } from '@/components/links/ShortLinkField'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { ApiError } from '@/lib/api'
import { formatShortUrl } from '@/lib/format'
import { useDomains, useEditBackHalf, useLink, useTags, useUpdateLink } from '@/lib/hooks'
import { normalizeDestination } from '@/lib/url'
import type { Link, UpdateLinkInput } from '@/types'

const BACK_HALF_TAKEN = 'That back-half is already taken'

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-4 rounded-xl bg-card px-6 pt-4 pb-6">
      <h2 className="text-lg font-semibold text-foreground">{title}</h2>
      {children}
    </section>
  )
}

function sameTags(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false
  const set = new Set(a)
  return b.every((t) => set.has(t))
}

/** The fields of PUT /links/:id that differ from the link; empty when nothing changed. */
function diffLink(
  link: Link,
  form: { title: string; tags: string[]; destination: string | null; expOn: boolean; expiresIso: string | null },
): UpdateLinkInput {
  const changes: UpdateLinkInput = {}
  const title = form.title.trim()
  if (title !== (link.title ?? '')) changes.title = title
  if (!sameTags(form.tags, link.tags ?? [])) changes.tags = form.tags
  if (form.destination && form.destination !== link.original_url) changes.original_url = form.destination
  if (form.expOn) {
    if (form.expiresIso && (!link.expires_at || new Date(link.expires_at).getTime() !== new Date(form.expiresIso).getTime())) {
      changes.expires_at = form.expiresIso
    }
  } else if (link.expires_at) {
    changes.clear_expiration = true
  }
  return changes
}

function EditLinkForm({ link }: { link: Link }) {
  const id = useId()
  const router = useRouter()
  const updateLink = useUpdateLink()
  const editBackHalf = useEditBackHalf()
  const { data: domains = [] } = useDomains()
  const { data: tagSuggestions = [] } = useTags()

  const [bannerOpen, setBannerOpen] = useState(true)
  const [editingBackHalf, setEditingBackHalf] = useState(false)
  const [domainId, setDomainId] = useState<number | null>(link.domain_id ?? null)
  const [backHalf, setBackHalf] = useState(link.short_code)
  const [backHalfError, setBackHalfError] = useState<string | null>(null)

  const [redirecting, setRedirecting] = useState(false)
  const [destination, setDestination] = useState(link.original_url)
  const [destinationError, setDestinationError] = useState<string | null>(null)

  const [title, setTitle] = useState(link.title ?? '')
  const [tags, setTags] = useState<string[]>(link.tags ?? [])
  const [expOn, setExpOn] = useState(link.expires_at != null)
  const [expiresAt, setExpiresAt] = useState(link.expires_at ? toDatetimeLocal(new Date(link.expires_at)) : '')
  const [expError, setExpError] = useState<string | null>(null)

  const saving = updateLink.isPending || editBackHalf.isPending
  const detailsHref = (linkId: number) => `/dashboard/links/${linkId}`

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (saving) return

    const bh = backHalf.trim()
    const backHalfChanged = editingBackHalf && (bh !== link.short_code || (domainId ?? null) !== (link.domain_id ?? null))
    if (backHalfChanged) {
      const err = bh ? validateBackHalf(bh) : 'Enter a back-half'
      if (err) {
        setBackHalfError(err)
        return
      }
    }
    setBackHalfError(null)

    let dest: string | null = null
    if (redirecting) {
      dest = normalizeDestination(destination)
      if (!dest) {
        setDestinationError('Enter a valid http(s) URL (up to 2048 characters)')
        return
      }
    }
    setDestinationError(null)

    let expiresIso: string | null = null
    if (expOn) {
      const d = new Date(expiresAt)
      if (!expiresAt || Number.isNaN(d.getTime())) {
        setExpError('Pick an expiration date')
        return
      }
      const unchanged = link.expires_at && new Date(link.expires_at).getTime() === d.getTime()
      if (!unchanged && d < minExpiration()) {
        setExpError('Pick a date at least 5 minutes from now')
        return
      }
      expiresIso = unchanged ? link.expires_at : d.toISOString()
    }
    setExpError(null)

    const changes = diffLink(link, { title, tags, destination: dest, expOn, expiresIso })
    const hasChanges = Object.keys(changes).length > 0

    if (backHalfChanged) {
      let created: Link
      try {
        created = await editBackHalf.mutateAsync({ id: link.id, domain_id: domainId ?? 0, back_half: bh })
      } catch (err) {
        if (err instanceof ApiError && err.status === 409) {
          setBackHalfError(BACK_HALF_TAKEN)
          toast.error(BACK_HALF_TAKEN)
        } else {
          toast.error(err instanceof Error ? err.message : "Couldn't create the new short link")
        }
        return
      }
      if (hasChanges) {
        try {
          await updateLink.mutateAsync({ id: created.id, data: changes })
        } catch (err) {
          toast.error(err instanceof Error ? `New link created, but: ${err.message}` : "Couldn't apply the other changes")
        }
      }
      toast.success('New short link created')
      router.push(detailsHref(created.id))
      return
    }

    if (!hasChanges) {
      router.push(detailsHref(link.id))
      return
    }
    try {
      await updateLink.mutateAsync({ id: link.id, data: changes })
      toast.success('Link updated')
      router.push(detailsHref(link.id))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Couldn't update the link")
    }
  }

  return (
    <form onSubmit={submit} noValidate>
      <div className="space-y-4 pb-24">
        <Section title="Short link">
          {bannerOpen && (
            <div className="flex items-start gap-3 rounded-lg bg-sidebar-active-bg px-4 py-3 text-sm text-foreground">
              <Info className="mt-0.5 size-5 shrink-0 text-sidebar-active-fg" aria-hidden />
              <p className="flex-1">
                Editing your short link will create a new, separate short link. The current short link will remain
                active and continue to point to the same destination.
              </p>
              <button
                type="button"
                aria-label="Dismiss"
                onClick={() => setBannerOpen(false)}
                className="shrink-0 rounded p-0.5 text-muted-foreground hover:bg-white/60 hover:text-foreground"
              >
                <X className="size-4" />
              </button>
            </div>
          )}
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="text-base font-semibold text-primary">{formatShortUrl(link.short_url)}</span>
            {!editingBackHalf && (
              <Button type="button" variant="link" className="h-auto px-0" onClick={() => setEditingBackHalf(true)}>
                <Pencil className="size-4" />
                Edit back-half
              </Button>
            )}
          </div>
          {editingBackHalf && (
            <ShortLinkField
              domains={domains}
              domainId={domainId}
              onDomainChange={setDomainId}
              backHalf={backHalf}
              onBackHalfChange={(v) => {
                setBackHalf(v)
                setBackHalfError(null)
              }}
              error={backHalfError}
            />
          )}
        </Section>

        <Section title="Destination URL">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="min-w-0 truncate text-base text-foreground" title={link.original_url}>
              {link.original_url}
            </span>
            {!redirecting && (
              <Button type="button" variant="link" className="h-auto px-0" onClick={() => setRedirecting(true)}>
                <CornerDownRight className="size-4" />
                Redirect
              </Button>
            )}
          </div>
          {redirecting && (
            <div className="space-y-1.5">
              <Label htmlFor={`${id}-destination`}>New destination URL</Label>
              <Input
                id={`${id}-destination`}
                type="url"
                value={destination}
                maxLength={2048}
                aria-invalid={destinationError ? true : undefined}
                onChange={(e) => {
                  setDestination(e.target.value)
                  setDestinationError(null)
                }}
              />
              {destinationError && <p className="text-sm text-destructive">{destinationError}</p>}
              <p className="text-sm text-muted-foreground">
                Changing the destination counts as a redirect. The link can no longer be deleted afterwards.
              </p>
            </div>
          )}
        </Section>

        <Section title="Optional details">
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-title`}>Title</Label>
            <Input id={`${id}-title`} value={title} maxLength={255} onChange={(e) => setTitle(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>Tags</Label>
            <TagPicker value={tags} onChange={setTags} suggestions={tagSuggestions} />
          </div>
          <div className="flex items-center gap-3">
            <Switch id={`${id}-exp`} checked={expOn} onCheckedChange={setExpOn} />
            <Label htmlFor={`${id}-exp`} className="font-semibold">
              Link expiration
            </Label>
          </div>
          {expOn && (
            <ExpirationField
              value={expiresAt}
              onChange={(v) => {
                setExpiresAt(v)
                setExpError(null)
              }}
              error={expError}
            />
          )}
        </Section>
      </div>

      <div className="sticky bottom-0 -mx-4 border-t border-border bg-background px-4 py-3">
        <div className="mx-auto flex max-w-[720px] justify-end gap-3">
          <Button type="button" variant="secondary" asChild>
            <NextLink href={detailsHref(link.id)}>Cancel</NextLink>
          </Button>
          <Button type="submit" disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </div>
    </form>
  )
}

export default function EditLinkPage() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)
  const { data: link, isLoading, isError } = useLink(Number.isFinite(id) ? id : null)

  return (
    <div className="mx-auto w-full max-w-[720px]">
      <PageHeader title="Edit link" backHref={Number.isFinite(id) ? `/dashboard/links/${id}` : '/dashboard/links'} />
      {isLoading ? (
        <div className="space-y-4">
          <Skeleton className="h-40 rounded-xl" />
          <Skeleton className="h-28 rounded-xl" />
          <Skeleton className="h-56 rounded-xl" />
        </div>
      ) : isError || !link ? (
        <EmptyState
          title="Link not found"
          description="It may have been deleted, or it belongs to another group."
          action={
            <Button asChild>
              <NextLink href="/dashboard/links">Back to links</NextLink>
            </Button>
          }
        />
      ) : (
        // Keyed so the form state re-initialises if the id changes.
        <EditLinkForm key={link.id} link={link} />
      )}
    </div>
  )
}
