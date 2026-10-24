'use client'

import type { ReactNode } from 'react'
import { ArrowLeft, Loader2, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { PageBlock, PageTemplate } from '@/types'
import { DESIGN_PRESETS } from './designPresets'
import { PagePreview } from './PagePreview'

type PresetTemplate = Exclude<PageTemplate, 'scratch'>

const TEMPLATES: { value: PresetTemplate; label: string }[] = [
  { value: 'classic', label: 'Classic' },
  { value: 'bold', label: 'Bold' },
  { value: 'minimal', label: 'Minimal' },
]

/** Client-side sample blocks for the mini previews (never sent to the server). */
const SAMPLE_BLOCKS: PageBlock[] = [
  {
    id: -1,
    type: 'text',
    position: 0,
    content: { heading: 'Welcome', body: 'This is your new page' },
    is_active: true,
    is_pinned: false,
    link_id: null,
  },
  {
    id: -2,
    type: 'link',
    position: 1,
    content: { title: 'My website', description: '', thumbnail_url: '' },
    is_active: true,
    is_pinned: false,
    link_id: null,
  },
]

const PREVIEW_W = 360
const PREVIEW_H = 480
const SCALE = 0.5

function Tile({
  label,
  busy,
  disabled,
  onClick,
  children,
}: {
  label: string
  busy: boolean
  disabled: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <div className="flex flex-col gap-2">
      <button
        type="button"
        onClick={onClick}
        disabled={disabled}
        aria-busy={busy || undefined}
        className={cn(
          'group relative overflow-hidden rounded-xl border border-border bg-card text-left outline-none transition-shadow',
          'hover:shadow-md focus-visible:ring-2 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-60',
        )}
        style={{ height: PREVIEW_H * SCALE }}
      >
        <span className="sr-only">{label}</span>
        <span aria-hidden className="block">
          {children}
        </span>
        {busy && (
          <span className="absolute inset-0 flex items-center justify-center bg-background/60">
            <Loader2 className="size-6 animate-spin text-primary" aria-hidden />
          </span>
        )}
      </button>
      <span aria-hidden className="text-center text-sm font-semibold text-foreground">
        {label}
      </span>
    </div>
  )
}

export interface TemplatePickerProps {
  slug: string
  /** The template being created (tiles are disabled meanwhile). */
  pending: PageTemplate | null
  onPick: (template: PageTemplate) => void
  onBack: () => void
}

/** Step 2 of page creation: "Start from scratch" or one of three preset templates with a mini preview. */
export function TemplatePicker({ slug, pending, onPick, onBack }: TemplatePickerProps) {
  const busy = pending != null
  return (
    <div className="flex flex-col gap-6 rounded-xl bg-card p-6">
      <div className="flex items-start justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h2 className="text-xl font-semibold text-foreground">Choose a template</h2>
          <p className="text-base text-muted-foreground">You can change the design any time.</p>
        </div>
        <Button type="button" variant="ghost" size="sm" onClick={onBack} disabled={busy}>
          <ArrowLeft />
          Back
        </Button>
      </div>
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Tile label="Start from scratch" busy={pending === 'scratch'} disabled={busy} onClick={() => onPick('scratch')}>
          <span className="flex flex-col items-center justify-center gap-2 text-muted-foreground" style={{ height: PREVIEW_H * SCALE }}>
            <Plus className="size-8" />
          </span>
        </Tile>
        {TEMPLATES.map((t) => (
          <Tile key={t.value} label={t.label} busy={pending === t.value} disabled={busy} onClick={() => onPick(t.value)}>
            {/* inert: the preview's links must not be focusable inside the tile button. */}
            <span
              inert
              className="pointer-events-none block origin-top-left"
              style={{ width: PREVIEW_W, height: PREVIEW_H, transform: `scale(${SCALE})` }}
            >
              <PagePreview
                page={{ title: slug, description: '', avatar_url: '', design: DESIGN_PRESETS[t.value], blocks: SAMPLE_BLOCKS }}
                mode="editor"
              />
            </span>
          </Tile>
        ))}
      </div>
    </div>
  )
}
