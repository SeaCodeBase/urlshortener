'use client'

import { useId, useRef, useState, type FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Textarea } from '@/components/ui/textarea'
import { formatShortUrl } from '@/lib/format'
import { useLinks } from '@/lib/hooks'
import { useDebouncedValue } from '@/lib/hooks/useDebouncedValue'
import { normalizeDestination } from '@/lib/url'
import { cn } from '@/lib/utils'
import type { Link } from '@/types'
import { charCount, Field, FormActions, invalidProps, isHttpUrl, tooLong, type BlockFormProps, type FieldErrors } from './blockFormShared'

const PAGE_SIZE = 50
const TITLE_MAX = 80
const DESCRIPTION_MAX = 160

type Mode = 'existing' | 'new'
type ErrorKey = 'link' | 'url' | 'title' | 'description' | 'thumbnail'

/** Searchable radio list of the group's links (only `source='link'` links are listed by the API). */
function LinkPicker({ value, onPick, error }: { value: number | null; onPick: (link: Link) => void; error?: string }) {
  const [search, setSearch] = useState('')
  const q = useDebouncedValue(search.trim(), 300)
  const { data, isLoading, hasNextPage, fetchNextPage, isFetchingNextPage } = useLinks({
    ...(q ? { q } : {}),
    limit: PAGE_SIZE,
  })
  const links = (data?.pages ?? []).flatMap((p) => p.links)

  return (
    <div className="space-y-2">
      <Input
        type="search"
        aria-label="Search links"
        placeholder="Search links"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <div className={cn('max-h-60 overflow-y-auto rounded-md border border-border p-1', error && 'border-destructive')}>
        {isLoading ? (
          <p className="py-4 text-center text-sm text-muted-foreground">Loading links…</p>
        ) : links.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">No links found.</p>
        ) : (
          <RadioGroup
            value={value == null ? '' : String(value)}
            onValueChange={(v) => {
              const link = links.find((l) => String(l.id) === v)
              if (link) onPick(link)
            }}
            aria-label="Links"
            className="gap-1"
          >
            {links.map((l) => (
              <div key={l.id} className="flex items-center gap-3 rounded-md px-2 py-2 hover:bg-muted">
                <RadioGroupItem id={`block-link-${l.id}`} value={String(l.id)} />
                <Label htmlFor={`block-link-${l.id}`} className="flex min-w-0 flex-1 flex-col items-start gap-0.5 font-normal">
                  <span className="w-full truncate font-medium text-foreground">{l.title || formatShortUrl(l.short_url)}</span>
                  <span className="w-full truncate text-xs text-muted-foreground">{formatShortUrl(l.short_url)}</span>
                </Label>
              </div>
            ))}
          </RadioGroup>
        )}
        {hasNextPage && (
          <Button
            type="button"
            variant="ghost"
            className="mt-1 w-full"
            disabled={isFetchingNextPage}
            onClick={() => fetchNextPage()}
          >
            Load more
          </Button>
        )}
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
    </div>
  )
}

function ModeToggle({ mode, onChange }: { mode: Mode; onChange: (m: Mode) => void }) {
  const options: { value: Mode; label: string }[] = [
    { value: 'existing', label: 'Existing link' },
    { value: 'new', label: 'New destination' },
  ]
  return (
    <div className="inline-flex rounded-md bg-muted p-1" role="group" aria-label="Link source">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={mode === o.value}
          onClick={() => onChange(o.value)}
          className={cn(
            'rounded px-3 py-1.5 text-sm font-medium text-muted-foreground',
            mode === o.value && 'bg-background text-foreground shadow-xs',
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

/** Link block: an existing link or a new destination, plus the button's title, description and thumbnail. */
export function LinkBlockForm({ block, submitting, onSubmit, onCancel }: BlockFormProps<'link'>) {
  const id = useId()
  const editing = block != null
  const [mode, setMode] = useState<Mode>('existing')
  const [linkId, setLinkId] = useState<number | null>(null)
  const [destination, setDestination] = useState('')
  const [linkTitle, setLinkTitle] = useState('')
  const [title, setTitle] = useState(block?.content.title ?? '')
  const [description, setDescription] = useState(block?.content.description ?? '')
  const [thumbnail, setThumbnail] = useState(block?.content.thumbnail_url ?? '')
  const [errors, setErrors] = useState<FieldErrors<ErrorKey>>({})
  // The title we last filled in from a picked link; a user-typed title is never overwritten.
  const autoTitle = useRef('')

  function pick(link: Link) {
    setLinkId(link.id)
    setErrors((e) => ({ ...e, link: undefined }))
    if (link.title && (title.trim() === '' || title === autoTitle.current)) {
      const next = link.title.slice(0, TITLE_MAX)
      autoTitle.current = next
      setTitle(next)
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    const t = title.trim()
    const d = description.trim()
    const th = thumbnail.trim()
    const next: FieldErrors<ErrorKey> = {}
    let original: string | null = null
    if (!editing) {
      if (mode === 'existing' && linkId == null) next.link = 'Pick a link or enter a destination URL'
      if (mode === 'new') {
        if (!destination.trim()) next.url = 'Pick a link or enter a destination URL'
        else if (!(original = normalizeDestination(destination))) next.url = 'Enter a valid URL'
      }
    }
    if (!t) next.title = 'Button title is required'
    else if (charCount(t) > TITLE_MAX) next.title = tooLong('Button title', TITLE_MAX)
    if (charCount(d) > DESCRIPTION_MAX) next.description = tooLong('Description', DESCRIPTION_MAX)
    if (th && !isHttpUrl(th)) next.thumbnail = 'Enter a valid URL'
    setErrors(next)
    if (Object.keys(next).length) return

    const content = { title: t, description: d, thumbnail_url: th }
    if (editing) onSubmit({ content })
    else if (mode === 'existing') onSubmit({ content, link_id: linkId as number })
    else {
      const lt = linkTitle.trim()
      onSubmit({ content, new_link: { original_url: original as string, ...(lt ? { title: lt } : {}) } })
    }
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-1 flex-col gap-4">
      {editing ? (
        <div className="space-y-1 rounded-md border border-border p-3 text-sm">
          <p className="font-medium text-foreground">{block.link?.title || 'Linked short link'}</p>
          {block.link && <p className="truncate text-primary">{formatShortUrl(block.link.short_url)}</p>}
          <p className="text-xs text-muted-foreground">Delete the block to change its link</p>
        </div>
      ) : (
        <>
          <ModeToggle
            mode={mode}
            onChange={(m) => {
              setMode(m)
              setErrors((e) => ({ ...e, link: undefined, url: undefined }))
            }}
          />
          {mode === 'existing' ? (
            <LinkPicker value={linkId} onPick={pick} error={errors.link} />
          ) : (
            <>
              <Field id={`${id}-dest`} label="Destination URL" error={errors.url}>
                <Input
                  id={`${id}-dest`}
                  placeholder="https://example.com/my-long-url"
                  value={destination}
                  onChange={(e) => setDestination(e.target.value)}
                  {...invalidProps(`${id}-dest`, errors.url)}
                />
              </Field>
              <Field id={`${id}-ltitle`} label="Link title (optional)">
                <Input id={`${id}-ltitle`} value={linkTitle} onChange={(e) => setLinkTitle(e.target.value)} />
              </Field>
            </>
          )}
        </>
      )}
      <Field id={`${id}-title`} label="Button title" error={errors.title}>
        <Input id={`${id}-title`} value={title} onChange={(e) => setTitle(e.target.value)} {...invalidProps(`${id}-title`, errors.title)} />
      </Field>
      <Field id={`${id}-desc`} label="Description" error={errors.description}>
        <Textarea
          id={`${id}-desc`}
          rows={3}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          {...invalidProps(`${id}-desc`, errors.description)}
        />
      </Field>
      <Field id={`${id}-thumb`} label="Thumbnail URL" error={errors.thumbnail}>
        <Input
          id={`${id}-thumb`}
          placeholder="https://"
          value={thumbnail}
          onChange={(e) => setThumbnail(e.target.value)}
          {...invalidProps(`${id}-thumb`, errors.thumbnail)}
        />
      </Field>
      <FormActions submitting={submitting} onCancel={onCancel} />
    </form>
  )
}
