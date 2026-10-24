'use client'

import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { toast } from 'sonner'
import { HEX_RE, QR_COLORS } from '@/components/links/LinkForm'
import { QrPreview } from '@/components/qr/QrPreview'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useUploadImage } from '@/lib/hooks'
import { MAX_LOGO_SIZE, MIN_LOGO_SIZE } from '@/lib/qr/design'
import { cn } from '@/lib/utils'
import {
  DEFAULT_QR_DESIGN,
  type QrCorners,
  type QrDesign,
  type QrErrorCorrection,
  type QrFrame,
  type QrPattern,
} from '@/types'

const PATTERNS: { value: QrPattern; label: string }[] = [
  { value: 'standard', label: 'Standard' },
  { value: 'rounded', label: 'Rounded' },
  { value: 'circle', label: 'Circle' },
  { value: 'diamond', label: 'Diamond' },
  { value: 'classy', label: 'Classy' },
]
const CORNERS: { value: QrCorners; label: string }[] = [
  { value: 'square', label: 'Square' },
  { value: 'rounded', label: 'Rounded' },
  { value: 'extra-rounded', label: 'Extra rounded' },
  { value: 'dot', label: 'Dot' },
]
const FRAMES: { value: QrFrame; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'border', label: 'Border' },
  { value: 'text-bottom', label: 'Text bottom' },
  { value: 'text-top', label: 'Text top' },
]
const ERROR_CORRECTIONS: { value: QrErrorCorrection; label: string }[] = [
  { value: 'L', label: 'Low (7%)' },
  { value: 'M', label: 'Medium (15%)' },
  { value: 'Q', label: 'Quartile (25%)' },
  { value: 'H', label: 'High (30%)' },
]
export const QR_BACKGROUNDS = ['#FFFFFF', '#F4F6FA', '#FFF8E1', '#E8F5E9', '#E3F2FD']
const MAX_FRAME_TEXT = 24

/** Enter in a designer text field must not submit an enclosing form (and create the code). */
function blockEnter(e: KeyboardEvent<HTMLInputElement>) {
  if (e.key === 'Enter') e.preventDefault()
}

function DesignSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-4 rounded-xl border border-border bg-card p-6">
      <h2 className="text-lg font-semibold text-foreground">{title}</h2>
      {children}
    </section>
  )
}

function Radios<T extends string>({
  label,
  options,
  value,
  onChange,
}: {
  label: string
  options: { value: T; label: string }[]
  value: T
  onChange: (value: T) => void
}) {
  const id = useId()
  return (
    <div className="space-y-2">
      <div id={id} className="text-sm font-semibold text-foreground">
        {label}
      </div>
      <RadioGroup
        aria-labelledby={id}
        value={value}
        onValueChange={(v) => onChange(v as T)}
        className="flex flex-wrap gap-x-5 gap-y-2"
      >
        {options.map((o) => (
          <div key={o.value} className="flex items-center gap-2">
            <RadioGroupItem id={`${id}-${o.value}`} value={o.value} />
            <Label htmlFor={`${id}-${o.value}`} className="font-normal">
              {o.label}
            </Label>
          </div>
        ))}
      </RadioGroup>
    </div>
  )
}

/** Swatches + a hex input; the input keeps what's typed and applies it once it's a valid #RRGGBB. */
function ColorPicker({
  label,
  swatches,
  value,
  onChange,
}: {
  label: string
  swatches: string[]
  value: string
  onChange: (color: string) => void
}) {
  const [hex, setHex] = useState(value)
  const [shown, setShown] = useState(value)
  if (shown !== value) {
    // The color changed elsewhere (a swatch, Reset): show it in the input.
    setShown(value)
    setHex(value)
  }
  return (
    <div className="space-y-2">
      <div className="text-sm font-semibold text-foreground">{label}</div>
      <div className="flex flex-wrap items-center gap-2">
        {swatches.map((c) => {
          const selected = value.toUpperCase() === c
          return (
            <button
              key={c}
              type="button"
              aria-label={`${label} ${c}`}
              aria-pressed={selected}
              onClick={() => onChange(c)}
              className={cn(
                'size-8 rounded-full border-2',
                selected ? 'border-primary ring-2 ring-primary/30' : 'border-border',
              )}
              style={{ backgroundColor: c }}
            />
          )
        })}
        <Input
          aria-label={`${label} hex`}
          className="w-28"
          value={hex}
          maxLength={7}
          onKeyDown={blockEnter}
          onChange={(e) => {
            const v = e.target.value
            setHex(v)
            if (HEX_RE.test(v)) onChange(v.toUpperCase())
          }}
        />
      </div>
    </div>
  )
}

/** "Customize design" step: style, colors, logo, frame and error correction with a live preview. */
export function QrDesigner({
  design,
  onChange,
  data,
}: {
  design: QrDesign
  onChange: (next: QrDesign) => void
  /** Content encoded in the preview. */
  data: string
}) {
  const id = useId()
  const upload = useUploadImage()
  const set = (patch: Partial<QrDesign>) => onChange({ ...design, ...patch })
  // The latest design, so an upload that finishes later doesn't revert edits made meanwhile.
  const latest = useRef(design)
  useEffect(() => {
    latest.current = design
  }, [design])
  const hasText = design.frame === 'text-bottom' || design.frame === 'text-top'

  async function uploadLogo(file: File | undefined) {
    if (!file) return
    try {
      const res = await upload.mutateAsync(file)
      onChange({ ...latest.current, logo: res.url })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Couldn't upload the logo")
    }
  }

  return (
    <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
      <div className="space-y-4">
        <DesignSection title="Select a style">
          <Radios label="Pattern" options={PATTERNS} value={design.pattern} onChange={(pattern) => set({ pattern })} />
          <Radios label="Corners" options={CORNERS} value={design.corners} onChange={(corners) => set({ corners })} />
        </DesignSection>

        <DesignSection title="Choose your colors">
          <ColorPicker label="Code color" swatches={QR_COLORS} value={design.color} onChange={(color) => set({ color })} />
          <ColorPicker
            label="Background color"
            swatches={QR_BACKGROUNDS}
            value={design.background}
            onChange={(background) => set({ background })}
          />
          <Button type="button" variant="link" className="h-auto px-0" onClick={() => onChange(DEFAULT_QR_DESIGN)}>
            Reset to default
          </Button>
        </DesignSection>

        <DesignSection title="Logo">
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-logo`}>Logo image (PNG or JPEG)</Label>
            <Input
              id={`${id}-logo`}
              type="file"
              accept="image/png,image/jpeg"
              disabled={upload.isPending}
              onChange={(e) => {
                void uploadLogo(e.target.files?.[0])
                e.target.value = ''
              }}
            />
            {upload.isPending && <p className="text-sm text-muted-foreground">Uploading…</p>}
          </div>
          {design.logo && (
            <div className="flex items-center gap-3">
              {/* eslint-disable-next-line @next/next/no-img-element -- uploaded image of unknown size */}
              <img src={design.logo} alt="Logo" className="size-12 rounded border border-border object-contain" />
              <Button type="button" variant="secondary" size="sm" onClick={() => set({ logo: null })}>
                Remove
              </Button>
            </div>
          )}
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-logo-size`}>Logo size ({Math.round(design.logo_size * 100)}%)</Label>
            <input
              id={`${id}-logo-size`}
              type="range"
              min={MIN_LOGO_SIZE * 100}
              max={MAX_LOGO_SIZE * 100}
              step={1}
              value={Math.round(design.logo_size * 100)}
              onChange={(e) => set({ logo_size: Number(e.target.value) / 100 })}
              className="w-full max-w-xs accent-primary"
            />
          </div>
        </DesignSection>

        <DesignSection title="Frame">
          <Radios label="Frame style" options={FRAMES} value={design.frame} onChange={(frame) => set({ frame })} />
          {hasText && (
            <div className="space-y-1.5">
              <Label htmlFor={`${id}-frame-text`}>Frame text</Label>
              <Input
                id={`${id}-frame-text`}
                className="sm:w-72"
                value={design.frame_text}
                maxLength={MAX_FRAME_TEXT}
                onKeyDown={blockEnter}
                onChange={(e) => set({ frame_text: e.target.value })}
              />
            </div>
          )}
          {design.frame !== 'none' && (
            <ColorPicker
              label="Frame color"
              swatches={QR_COLORS}
              value={design.frame_color}
              onChange={(frame_color) => set({ frame_color })}
            />
          )}
        </DesignSection>

        <DesignSection title="Advanced">
          <div className="space-y-1.5 sm:w-56">
            <Label htmlFor={`${id}-ec`}>Error correction</Label>
            <Select
              value={design.error_correction}
              onValueChange={(v) => set({ error_correction: v as QrErrorCorrection })}
            >
              <SelectTrigger id={`${id}-ec`} className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ERROR_CORRECTIONS.map((o) => (
                  <SelectItem key={o.value} value={o.value}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-sm text-muted-foreground">Higher levels keep the code scannable with a logo.</p>
          </div>
        </DesignSection>
      </div>

      <aside className="lg:sticky lg:top-4 lg:self-start">
        <div className="space-y-3 rounded-xl border border-border bg-card p-6">
          <h2 className="text-lg font-semibold text-foreground">Preview</h2>
          <div className="flex justify-center">
            <QrPreview design={design} data={data} size={240} />
          </div>
        </div>
      </aside>
    </div>
  )
}
