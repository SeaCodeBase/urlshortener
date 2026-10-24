'use client'

import { useId, useState, type FormEvent } from 'react'
import { Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { normalizeDestination } from '@/lib/url'
import type { SocialItem, SocialPlatform } from '@/types'
import { FormActions } from './blockFormShared'
import { SOCIAL_PLATFORMS, SOCIAL_PLATFORM_KEYS } from './designPresets'
import type { BlockFormProps } from './blockFormShared'

export const SOCIAL_MAX = 10
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

/** The URL to store for a row, or null when it isn't valid (bare emails become `mailto:`, bare hosts get https://). */
export function normalizeSocialUrl(platform: SocialPlatform, raw: string): string | null {
  const v = raw.trim()
  if (!v) return null
  if (platform === 'email') {
    const addr = v.toLowerCase().startsWith('mailto:') ? v.slice(7) : v
    return EMAIL_RE.test(addr) ? `mailto:${addr}` : null
  }
  return normalizeDestination(v)
}

const placeholder = (p: SocialPlatform) => (p === 'email' ? 'you@example.com' : 'https://')

/** Social icons block: up to 10 platform + URL rows. */
export function SocialBlockForm({ block, submitting, onSubmit, onCancel }: BlockFormProps<'social'>) {
  const id = useId()
  const [items, setItems] = useState<SocialItem[]>(block?.content.items ?? [])
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({})
  const [formError, setFormError] = useState('')

  const setItem = (i: number, patch: Partial<SocialItem>) =>
    setItems((cur) => cur.map((it, j) => (j === i ? { ...it, ...patch } : it)))

  function addRow() {
    const used = new Set(items.map((it) => it.platform))
    const platform = SOCIAL_PLATFORM_KEYS.find((p) => !used.has(p)) ?? 'website'
    setItems((cur) => [...cur, { platform, url: '' }])
    setFormError('')
  }

  function removeRow(i: number) {
    setItems((cur) => cur.filter((_, j) => j !== i))
    setRowErrors({})
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    if (items.length === 0) {
      setFormError('Add at least one profile')
      return
    }
    const errors: Record<number, string> = {}
    const out: SocialItem[] = items.map((it, i) => {
      const url = normalizeSocialUrl(it.platform, it.url)
      if (!url) errors[i] = it.platform === 'email' ? 'Enter a valid email address' : 'Enter a valid URL'
      return { platform: it.platform, url: url ?? '' }
    })
    setRowErrors(errors)
    setFormError('')
    if (Object.keys(errors).length) return
    onSubmit({ content: { items: out } })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-1 flex-col gap-4">
      <ul className="space-y-3">
        {items.map((it, i) => (
          <li key={i} className="space-y-1">
            <div className="flex items-center gap-2">
              <Select value={it.platform} onValueChange={(v) => setItem(i, { platform: v as SocialPlatform })}>
                <SelectTrigger aria-label={`Platform ${i + 1}`} className="w-36 shrink-0">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SOCIAL_PLATFORM_KEYS.map((p) => {
                    const { label, icon: Icon } = SOCIAL_PLATFORMS[p]
                    return (
                      <SelectItem key={p} value={p}>
                        <Icon aria-hidden />
                        {label}
                      </SelectItem>
                    )
                  })}
                </SelectContent>
              </Select>
              <Input
                id={`${id}-url-${i}`}
                aria-label={`Profile URL ${i + 1}`}
                placeholder={placeholder(it.platform)}
                value={it.url}
                onChange={(e) => setItem(i, { url: e.target.value })}
                aria-invalid={rowErrors[i] ? true : undefined}
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={`Remove profile ${i + 1}`}
                onClick={() => removeRow(i)}
              >
                <X />
              </Button>
            </div>
            {rowErrors[i] && <p className="text-sm text-destructive">{rowErrors[i]}</p>}
          </li>
        ))}
      </ul>
      <Button
        type="button"
        variant="secondary"
        className="self-start"
        disabled={items.length >= SOCIAL_MAX}
        onClick={addRow}
      >
        <Plus />
        Add profile
      </Button>
      {items.length >= SOCIAL_MAX && <p className="text-xs text-muted-foreground">Up to {SOCIAL_MAX} profiles.</p>}
      {formError && <p className="text-sm text-destructive">{formError}</p>}
      <FormActions submitting={submitting} onCancel={onCancel} />
    </form>
  )
}
