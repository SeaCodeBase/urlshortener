'use client'

import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { Upload } from 'lucide-react'
import { toast } from 'sonner'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { usePageMutations, useUploadImage } from '@/lib/hooks'
import { cn } from '@/lib/utils'
import type { Page, PageButtonRadius, PageButtonStyle, PageDesign, PageFont, PageBackgroundType } from '@/types'
import { charCount, invalidProps, isHttpUrl, tooLong } from './blockFormShared'
import { DESIGN_PRESETS, PRESET_THEMES } from './designPresets'
import { PagePreview } from './PagePreview'

/** Design saves are debounced this long (presets save at once). */
export const DESIGN_SAVE_DELAY_MS = 400
export const HEX_RE = /^#[0-9a-fA-F]{6}$/
export const HEX_ERROR = 'Enter a colour like #2A5BD7'

const TITLE_MAX = 255
const BIO_MAX = 500
const AVATAR_MAX = 512

const THEME_LABEL: Record<string, string> = {
  classic: 'Classic',
  bold: 'Bold',
  minimal: 'Minimal',
  dark: 'Dark',
  sunset: 'Sunset',
}

/** Same rule as the backend: http(s) or a path from the uploads endpoint. */
const isAvatarUrl = (v: string) => isHttpUrl(v) || (/^\/uploads\/[^/\s]+$/.test(v) && !v.includes('..'))

type ColourKey = 'bg' | 'bg2' | 'text' | 'btn' | 'btnText'

function getColour(d: PageDesign, k: ColourKey): string {
  switch (k) {
    case 'bg':
      return d.background.color
    case 'bg2':
      return d.background.color2
    case 'text':
      return d.text_color
    case 'btn':
      return d.button.color
    case 'btnText':
      return d.button.text_color
  }
}

function setColour(d: PageDesign, k: ColourKey, v: string): PageDesign {
  switch (k) {
    case 'bg':
      return { ...d, background: { ...d.background, color: v } }
    case 'bg2':
      return { ...d, background: { ...d.background, color2: v } }
    case 'text':
      return { ...d, text_color: v }
    case 'btn':
      return { ...d, button: { ...d.button, color: v } }
    case 'btnText':
      return { ...d, button: { ...d.button, text_color: v } }
  }
}

const COLOUR_KEYS: ColourKey[] = ['bg', 'bg2', 'text', 'btn', 'btnText']
const hexTexts = (d: PageDesign) => Object.fromEntries(COLOUR_KEYS.map((k) => [k, getColour(d, k)])) as Record<ColourKey, string>

function Card({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-4 rounded-xl bg-card p-6">
      <h3 className="text-lg font-semibold text-foreground">{title}</h3>
      {children}
    </section>
  )
}

/** A row of `aria-pressed` buttons, one selected. */
function Segmented<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: T
  options: { value: T; label: string }[]
  onChange: (v: T) => void
}) {
  return (
    <div className="space-y-1.5">
      <div className="text-sm font-medium text-foreground">{label}</div>
      <div role="group" aria-label={label} className="inline-flex rounded-md border border-border bg-white p-0.5 dark:bg-card">
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            aria-pressed={value === o.value}
            onClick={() => value !== o.value && onChange(o.value)}
            className={cn(
              'rounded px-3 py-1.5 text-sm font-medium transition-colors',
              value === o.value ? 'bg-primary text-primary-foreground' : 'text-foreground hover:bg-muted',
            )}
          >
            {o.label}
          </button>
        ))}
      </div>
    </div>
  )
}

/** Native colour picker + hex text input; an invalid hex shows an inline error. */
function ColourField({
  label,
  text,
  error,
  onText,
}: {
  label: string
  text: string
  error?: string
  onText: (v: string) => void
}) {
  const id = useId()
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      <div className="flex items-center gap-2">
        <input
          type="color"
          aria-label={`${label} picker`}
          value={HEX_RE.test(text) ? text.toLowerCase() : '#000000'}
          onChange={(e) => onText(e.target.value.toUpperCase())}
          className="h-10 w-12 shrink-0 cursor-pointer rounded border border-border bg-white p-1"
        />
        <Input
          id={id}
          value={text}
          maxLength={7}
          spellCheck={false}
          className="w-32 font-mono"
          onChange={(e) => onText(e.target.value.trim())}
          {...invalidProps(id, error)}
        />
      </div>
      {error && (
        <p id={`${id}-error`} className="text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}

const SAMPLE_BLOCKS = [
  {
    id: -1,
    type: 'link' as const,
    position: 0,
    content: { title: 'My link', description: '', thumbnail_url: '' },
    is_active: true,
    is_pinned: false,
    link_id: null,
  },
  {
    id: -2,
    type: 'link' as const,
    position: 1,
    content: { title: 'Another link', description: '', thumbnail_url: '' },
    is_active: true,
    is_pinned: false,
    link_id: null,
  },
]

export interface DesignTabProps {
  page: Page
  /** The unsaved design to preview, or null once there is nothing pending (preview falls back to the page). */
  onDesignChange?: (design: PageDesign | null) => void
}

/** Design tab of the page editor (spec §5.7): profile, theme presets, colours, buttons and font. */
export function DesignTab({ page, onDesignChange }: DesignTabProps) {
  const { update } = usePageMutations()
  const upload = useUploadImage()
  const id = useId()

  const [design, setDesign] = useState<PageDesign>(page.design)
  const [texts, setTexts] = useState(() => hexTexts(page.design))
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pending = useRef<PageDesign | null>(null)
  const saveSeq = useRef(0)

  const [title, setTitle] = useState(page.title)
  const [bio, setBio] = useState(page.description)
  const [avatar, setAvatar] = useState(page.avatar_url)
  const [profileErrors, setProfileErrors] = useState<{ title?: string; bio?: string; avatar?: string }>({})

  // Latest values for the unmount flush.
  const latest = useRef({ update, pageId: page.id, onDesignChange })
  useEffect(() => {
    latest.current = { update, pageId: page.id, onDesignChange }
  })

  function save(next: PageDesign) {
    pending.current = null
    const seq = ++saveSeq.current
    update.mutate(
      { id: page.id, patch: { design: next } },
      {
        onError: (err) => {
          if (seq !== saveSeq.current) return
          toast.error(err instanceof Error ? err.message : "Couldn't save the design")
          setDesign(page.design)
          setTexts(hexTexts(page.design))
          onDesignChange?.(null)
        },
      },
    )
  }

  // Leaving the tab with an edit still waiting: save it now rather than drop it.
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current)
      const next = pending.current
      if (!next) return
      const { update: u, pageId, onDesignChange: notify } = latest.current
      u.mutate(
        { id: pageId, patch: { design: next } },
        {
          onError: (err) => {
            toast.error(err instanceof Error ? err.message : "Couldn't save the design")
            notify?.(null)
          },
        },
      )
    },
    [],
  )

  function apply(next: PageDesign, immediate = false) {
    setDesign(next)
    onDesignChange?.(next)
    if (timer.current) clearTimeout(timer.current)
    timer.current = null
    if (immediate) {
      save(next)
      return
    }
    pending.current = next
    timer.current = setTimeout(() => {
      timer.current = null
      save(next)
    }, DESIGN_SAVE_DELAY_MS)
  }

  function onColour(k: ColourKey, v: string) {
    setTexts((t) => ({ ...t, [k]: v }))
    if (HEX_RE.test(v)) apply(setColour(design, k, v))
  }

  const hexError = (k: ColourKey) => (HEX_RE.test(texts[k]) ? undefined : HEX_ERROR)

  function applyPreset(theme: keyof typeof DESIGN_PRESETS) {
    const next = DESIGN_PRESETS[theme]
    setTexts(hexTexts(next))
    apply(next, true)
  }

  function saveProfile(patch: { title?: string; description?: string; avatar_url?: string }, revert: () => void) {
    update.mutate(
      { id: page.id, patch },
      {
        onError: (err) => {
          toast.error(err instanceof Error ? err.message : "Couldn't save the profile")
          revert()
        },
      },
    )
  }

  function blurTitle() {
    const t = title.trim()
    if (!t) {
      setTitle(page.title)
      setProfileErrors((e) => ({ ...e, title: undefined }))
      return
    }
    if (charCount(t) > TITLE_MAX) return setProfileErrors((e) => ({ ...e, title: tooLong('Display name', TITLE_MAX) }))
    setProfileErrors((e) => ({ ...e, title: undefined }))
    if (t !== page.title) saveProfile({ title: t }, () => setTitle(page.title))
  }

  function blurBio() {
    const b = bio.trim()
    if (charCount(b) > BIO_MAX) return setProfileErrors((e) => ({ ...e, bio: tooLong('Bio', BIO_MAX) }))
    setProfileErrors((e) => ({ ...e, bio: undefined }))
    if (b !== page.description) saveProfile({ description: b }, () => setBio(page.description))
  }

  function commitAvatar(value: string) {
    const a = value.trim()
    if (a && (a.length > AVATAR_MAX || !isAvatarUrl(a))) {
      return setProfileErrors((e) => ({ ...e, avatar: 'Enter a valid image URL' }))
    }
    setProfileErrors((e) => ({ ...e, avatar: undefined }))
    if (a !== page.avatar_url) saveProfile({ avatar_url: a }, () => setAvatar(page.avatar_url))
  }

  async function uploadAvatar(file: File | undefined) {
    if (!file) return
    try {
      const res = await upload.mutateAsync(file)
      setAvatar(res.url)
      commitAvatar(res.url)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Couldn't upload the image")
    }
  }

  const avatarPreview = avatar.trim() && isAvatarUrl(avatar.trim()) ? avatar.trim() : ''

  return (
    <div className="space-y-4">
      <Card title="Profile">
        <div className="flex items-start gap-4">
          {avatarPreview ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={avatarPreview} alt="Avatar preview" className="size-16 shrink-0 rounded-full border border-border object-cover" />
          ) : (
            <div
              aria-hidden
              className="flex size-16 shrink-0 items-center justify-center rounded-full bg-muted text-xl font-semibold text-muted-foreground"
            >
              {(title.trim() || page.slug).charAt(0).toUpperCase()}
            </div>
          )}
          <div className="min-w-0 flex-1 space-y-1.5">
            <Label htmlFor={`${id}-avatar`}>Avatar URL</Label>
            <div className="flex gap-2">
              <Input
                id={`${id}-avatar`}
                placeholder="https://"
                value={avatar}
                onChange={(e) => setAvatar(e.target.value)}
                onBlur={() => commitAvatar(avatar)}
                {...invalidProps(`${id}-avatar`, profileErrors.avatar)}
              />
              <Label
                htmlFor={`${id}-file`}
                className={cn(
                  'inline-flex h-10 shrink-0 cursor-pointer items-center gap-2 rounded border border-border bg-white px-3 text-base font-medium hover:bg-muted dark:bg-card',
                  upload.isPending && 'pointer-events-none opacity-50',
                )}
              >
                <Upload className="size-4" aria-hidden />
                Upload
              </Label>
              <input
                id={`${id}-file`}
                type="file"
                accept="image/png,image/jpeg,image/gif,image/webp"
                className="sr-only"
                disabled={upload.isPending}
                onChange={(e) => {
                  void uploadAvatar(e.target.files?.[0])
                  e.target.value = ''
                }}
              />
            </div>
            {profileErrors.avatar && (
              <p id={`${id}-avatar-error`} className="text-sm text-destructive">
                {profileErrors.avatar}
              </p>
            )}
          </div>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor={`${id}-title`}>Display name</Label>
          <Input
            id={`${id}-title`}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onBlur={blurTitle}
            {...invalidProps(`${id}-title`, profileErrors.title)}
          />
          {profileErrors.title && (
            <p id={`${id}-title-error`} className="text-sm text-destructive">
              {profileErrors.title}
            </p>
          )}
        </div>
        <div className="space-y-1.5">
          <Label htmlFor={`${id}-bio`}>Bio</Label>
          <Textarea
            id={`${id}-bio`}
            rows={3}
            value={bio}
            onChange={(e) => setBio(e.target.value)}
            onBlur={blurBio}
            {...invalidProps(`${id}-bio`, profileErrors.bio)}
          />
          {profileErrors.bio && (
            <p id={`${id}-bio-error`} className="text-sm text-destructive">
              {profileErrors.bio}
            </p>
          )}
        </div>
      </Card>

      <Card title="Theme">
        <div className="grid grid-cols-3 gap-3 sm:grid-cols-5">
          {PRESET_THEMES.map((theme) => (
            <button
              key={theme}
              type="button"
              aria-pressed={design.theme === theme}
              aria-label={THEME_LABEL[theme]}
              onClick={() => applyPreset(theme)}
              className={cn(
                'group flex flex-col items-center gap-2 rounded-lg p-1 text-sm font-medium text-foreground',
                design.theme === theme ? 'ring-2 ring-primary' : 'hover:bg-muted',
              )}
            >
              <span aria-hidden className="pointer-events-none block h-[140px] w-[94px] overflow-hidden rounded-md border border-border">
                <span className="block w-[375px] origin-top-left scale-[0.25]" style={{ height: 560 }}>
                  <PagePreview
                    mode="editor"
                    page={{ title: THEME_LABEL[theme], description: '', avatar_url: '', design: DESIGN_PRESETS[theme], blocks: SAMPLE_BLOCKS }}
                  />
                </span>
              </span>
              <span aria-hidden>{THEME_LABEL[theme]}</span>
            </button>
          ))}
        </div>
      </Card>

      <Card title="Background">
        <Segmented<PageBackgroundType>
          label="Background type"
          value={design.background.type}
          options={[
            { value: 'solid', label: 'Solid' },
            { value: 'gradient', label: 'Gradient' },
          ]}
          onChange={(type) => apply({ ...design, background: { ...design.background, type } })}
        />
        <div className="flex flex-wrap gap-6">
          <ColourField
            label={design.background.type === 'gradient' ? 'Gradient start' : 'Background colour'}
            text={texts.bg}
            error={hexError('bg')}
            onText={(v) => onColour('bg', v)}
          />
          {design.background.type === 'gradient' && (
            <ColourField label="Gradient end" text={texts.bg2} error={hexError('bg2')} onText={(v) => onColour('bg2', v)} />
          )}
        </div>
      </Card>

      <Card title="Text colour">
        <ColourField label="Text colour" text={texts.text} error={hexError('text')} onText={(v) => onColour('text', v)} />
      </Card>

      <Card title="Buttons">
        <div className="flex flex-wrap gap-6">
          <Segmented<PageButtonStyle>
            label="Button style"
            value={design.button.style}
            options={[
              { value: 'fill', label: 'Fill' },
              { value: 'outline', label: 'Outline' },
            ]}
            onChange={(style) => apply({ ...design, button: { ...design.button, style } })}
          />
          <Segmented<PageButtonRadius>
            label="Button shape"
            value={design.button.radius}
            options={[
              { value: 'square', label: 'Square' },
              { value: 'rounded', label: 'Rounded' },
              { value: 'pill', label: 'Pill' },
            ]}
            onChange={(radius) => apply({ ...design, button: { ...design.button, radius } })}
          />
        </div>
        <div className="flex flex-wrap gap-6">
          <ColourField label="Button colour" text={texts.btn} error={hexError('btn')} onText={(v) => onColour('btn', v)} />
          <ColourField
            label="Button text colour"
            text={texts.btnText}
            error={hexError('btnText')}
            onText={(v) => onColour('btnText', v)}
          />
        </div>
      </Card>

      <Card title="Font">
        <Segmented<PageFont>
          label="Font"
          value={design.font}
          options={[
            { value: 'inter', label: 'Inter' },
            { value: 'serif', label: 'Serif' },
            { value: 'mono', label: 'Mono' },
          ]}
          onChange={(font) => apply({ ...design, font })}
        />
      </Card>
    </div>
  )
}
