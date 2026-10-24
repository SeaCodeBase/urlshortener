'use client'

import { useId, useState } from 'react'
import { Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { UTMParams } from '@/types'

export type UtmField = 'source' | 'medium' | 'campaign' | 'term' | 'content'

export const UTM_FIELDS: { key: UtmField; label: string; placeholder: string }[] = [
  { key: 'source', label: 'Source', placeholder: 'e.g. google, newsletter' },
  { key: 'medium', label: 'Medium', placeholder: 'e.g. cpc, email' },
  { key: 'campaign', label: 'Campaign', placeholder: 'e.g. spring_sale' },
  { key: 'term', label: 'Term', placeholder: 'e.g. running+shoes' },
  { key: 'content', label: 'Content', placeholder: 'e.g. logolink' },
]

const HISTORY_KEY = 'utm-history'
const HISTORY_MAX = 10

type UtmHistory = Partial<Record<UtmField, string[]>>

/**
 * The destination with the UTM and custom parameters applied: appends or overwrites
 * query params, preserves the fragment and skips empty values. "" for an invalid URL.
 */
export function buildUtmPreview(destination: string, utm: UTMParams): string {
  let url: URL
  try {
    url = new URL(destination.trim())
  } catch {
    return ''
  }
  for (const { key } of UTM_FIELDS) {
    const v = utm[key]?.trim()
    if (v) url.searchParams.set(`utm_${key}`, v)
  }
  for (const { key, value } of utm.custom ?? []) {
    const k = key.trim()
    const v = value.trim()
    if (k && v) url.searchParams.set(k, v)
  }
  return url.toString()
}

/** UTM params with blanks dropped; undefined when nothing is set. */
export function cleanUtm(utm: UTMParams): UTMParams | undefined {
  const out: UTMParams = {}
  for (const { key } of UTM_FIELDS) {
    const v = utm[key]?.trim()
    if (v) out[key] = v
  }
  const custom = (utm.custom ?? [])
    .map((c) => ({ key: c.key.trim(), value: c.value.trim() }))
    .filter((c) => c.key && c.value)
  if (custom.length) out.custom = custom
  return Object.keys(out).length ? out : undefined
}

function readHistory(): UtmHistory {
  try {
    const raw = localStorage.getItem(HISTORY_KEY)
    const parsed: unknown = raw ? JSON.parse(raw) : {}
    return parsed && typeof parsed === 'object' ? (parsed as UtmHistory) : {}
  } catch {
    return {}
  }
}

/** Stores the used values (most recent first) as suggestions for next time. */
export function rememberUtm(utm: UTMParams) {
  const history = readHistory()
  for (const { key } of UTM_FIELDS) {
    const v = utm[key]?.trim()
    if (!v) continue
    history[key] = [v, ...(history[key] ?? []).filter((x) => x !== v)].slice(0, HISTORY_MAX)
  }
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history))
  } catch {
    // Storage unavailable: suggestions are a convenience only.
  }
}

/** UTM fields with remembered suggestions, custom params and a live preview. */
export function UtmBuilder({
  value,
  onChange,
  destination,
}: {
  value: UTMParams
  onChange: (utm: UTMParams) => void
  destination: string
}) {
  const id = useId()
  const [history] = useState<UtmHistory>(readHistory)
  const custom = value.custom ?? []
  const preview = buildUtmPreview(destination, value)

  function setCustom(next: typeof custom) {
    onChange({ ...value, custom: next })
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        {UTM_FIELDS.map(({ key, label, placeholder }) => (
          <div key={key} className="space-y-1.5">
            <Label htmlFor={`${id}-${key}`}>{label}</Label>
            <Input
              id={`${id}-${key}`}
              list={`${id}-${key}-list`}
              placeholder={placeholder}
              value={value[key] ?? ''}
              onChange={(e) => onChange({ ...value, [key]: e.target.value })}
            />
            <datalist id={`${id}-${key}-list`}>
              {(history[key] ?? []).map((v) => (
                <option key={v} value={v} />
              ))}
            </datalist>
          </div>
        ))}
      </div>

      {custom.map((row, i) => (
        <div key={i} className="flex items-end gap-2">
          <div className="flex-1 space-y-1.5">
            <Label htmlFor={`${id}-ck-${i}`}>Parameter</Label>
            <Input
              id={`${id}-ck-${i}`}
              placeholder="key"
              value={row.key}
              onChange={(e) => setCustom(custom.map((c, j) => (j === i ? { ...c, key: e.target.value } : c)))}
            />
          </div>
          <div className="flex-1 space-y-1.5">
            <Label htmlFor={`${id}-cv-${i}`}>Value</Label>
            <Input
              id={`${id}-cv-${i}`}
              placeholder="value"
              value={row.value}
              onChange={(e) => setCustom(custom.map((c, j) => (j === i ? { ...c, value: e.target.value } : c)))}
            />
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label="Remove parameter"
            onClick={() => setCustom(custom.filter((_, j) => j !== i))}
          >
            <X className="size-4" />
          </Button>
        </div>
      ))}

      <Button
        type="button"
        variant="link"
        className="h-auto px-0"
        onClick={() => setCustom([...custom, { key: '', value: '' }])}
      >
        <Plus className="size-4" />
        Add custom URL parameter
      </Button>

      <div className="space-y-1.5">
        <div className="text-sm font-semibold text-foreground">UTM tag preview</div>
        <div className="rounded bg-muted px-3 py-2 text-sm break-all text-muted-foreground">
          {preview || 'Enter a valid destination URL to see the preview.'}
        </div>
      </div>
    </div>
  )
}
