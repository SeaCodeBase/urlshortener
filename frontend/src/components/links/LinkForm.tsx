'use client'

import { useId, useState, type FormEvent, type ReactNode } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { ChevronDown } from 'lucide-react'
import { toast } from 'sonner'
import { TagPicker } from '@/components/common/TagPicker'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { ApiError } from '@/lib/api'
import { hostOf } from '@/lib/format'
import {
  useActiveGroupId,
  useCreateLink,
  useCreateQrCode,
  useDomains,
  usePageMutations,
  usePages,
  useTags,
  useUploadImage,
} from '@/lib/hooks'
import { normalizeDestination } from '@/lib/url'
import { cn } from '@/lib/utils'
import { DEFAULT_QR_DESIGN, type CreateLinkInput, type Domain, type UTMParams } from '@/types'
import { BUTTON_TITLE_MAX, NoPagesYet, PageSelect } from './AddToPageDialog'
import { ExpirationField, minExpiration } from './ExpirationField'
import { RoutingRulesEditor, emptyRule, toRoutingRuleInputs, type RoutingRuleDraft } from './RoutingRulesEditor'
import {
  ShortLinkField,
  domainIdForCreate,
  usableDomains,
  useGroupDefaultDomainId,
  validateBackHalf,
} from './ShortLinkField'
import { UtmBuilder, cleanUtm, rememberUtm } from './UtmBuilder'

export const QR_COLORS = ['#000000', '#D93025', '#F3A33B', '#1E8E3E', '#61B3DE', '#2A5BD7', '#8C6BE0', '#D64680']
export const HEX_RE = /^#[0-9A-Fa-f]{6}$/
const LOGO_TYPES = ['image/png', 'image/jpeg']
const MAX_LOGO_BYTES = 5 * 1024 * 1024

export function Section({ title, children }: { title: string; children: ReactNode }) {
  const [open, setOpen] = useState(true)
  const id = useId()
  return (
    <section className="rounded-xl border border-border bg-card">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between px-6 py-4 text-left"
      >
        <h2 className="text-lg font-semibold text-foreground">{title}</h2>
        <ChevronDown className={cn('size-5 text-muted-foreground transition-transform', open && 'rotate-180')} />
      </button>
      <div id={id} hidden={!open} className="space-y-5 px-6 pb-6">
        {children}
      </div>
    </section>
  )
}

export function SwitchRow({
  label,
  checked,
  onCheckedChange,
  disabled,
}: {
  label: string
  checked: boolean
  onCheckedChange: (v: boolean) => void
  disabled?: boolean
}) {
  const id = useId()
  return (
    <div className="flex items-center gap-3">
      <Switch id={id} checked={checked} onCheckedChange={onCheckedChange} disabled={disabled} />
      <Label htmlFor={id} className={cn('font-semibold', disabled && 'text-muted-foreground')}>
        {label}
      </Label>
    </div>
  )
}

/** Create-link form (spec §5.3): Link details, Sharing options, Advanced settings. */
export function LinkForm({ mode = 'create', defaultDomains }: { mode?: 'create'; defaultDomains?: Domain[] }) {
  const id = useId()
  const router = useRouter()
  const createLink = useCreateLink()
  const createQr = useCreateQrCode()
  const upload = useUploadImage()
  const { addBlock } = usePageMutations()
  const { data: fetchedDomains } = useDomains()
  const { data: tagSuggestions } = useTags()
  const domains = defaultDomains ?? fetchedDomains ?? []

  const [url, setUrl] = useState('')
  const [urlError, setUrlError] = useState<string | null>(null)
  // undefined until the user picks a domain: the group default is shown.
  const [domainChoice, setDomainChoice] = useState<number | null | undefined>(undefined)
  const activeGroupId = useActiveGroupId()
  const groupDefaultDomainId = useGroupDefaultDomainId(usableDomains(domains, activeGroupId))
  const domainId = domainChoice === undefined ? groupDefaultDomainId : domainChoice
  const [backHalf, setBackHalf] = useState('')
  const [backHalfError, setBackHalfError] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [tags, setTags] = useState<string[]>([])

  const [qrOn, setQrOn] = useState(false)
  const [qrColor, setQrColor] = useState(QR_COLORS[0])
  const [qrHex, setQrHex] = useState(QR_COLORS[0])
  /** Uploaded logo URL. */
  const [qrLogo, setQrLogo] = useState<string | null>(null)
  const [qrLogoName, setQrLogoName] = useState<string | null>(null)

  const [pageOn, setPageOn] = useState(false)
  const [pageChoice, setPageChoice] = useState<string | null>(null)
  /** null until edited: follows the destination host. */
  const [buttonTitleEdit, setButtonTitleEdit] = useState<string | null>(null)
  const [buttonTitleError, setButtonTitleError] = useState<string | null>(null)
  const { data: pagesData } = usePages()
  const pages = pagesData?.pages ?? []
  const pageId = pageChoice ?? (pages[0] ? String(pages[0].id) : '')
  const destinationHost = url.trim() ? hostOf(normalizeDestination(url) ?? url.trim()) : ''
  const buttonTitle = buttonTitleEdit ?? destinationHost.slice(0, BUTTON_TITLE_MAX)

  const [utmOn, setUtmOn] = useState(false)
  const [utm, setUtm] = useState<UTMParams>({})
  const [expOn, setExpOn] = useState(false)
  const [expiresAt, setExpiresAt] = useState('')
  const [expError, setExpError] = useState<string | null>(null)
  const [routingOn, setRoutingOn] = useState(false)
  const [rules, setRules] = useState<RoutingRuleDraft[]>([emptyRule()])

  async function uploadLogo(file: File | undefined) {
    if (!file) return
    if (!LOGO_TYPES.includes(file.type) || file.size > MAX_LOGO_BYTES) {
      toast.error('Logo must be a PNG or JPEG up to 5 MB')
      return
    }
    try {
      const res = await upload.mutateAsync(file)
      setQrLogo(res.url)
      setQrLogoName(file.name)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Couldn't upload the logo")
    }
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (createLink.isPending || createQr.isPending || addBlock.isPending || upload.isPending) return

    const destination = normalizeDestination(url)
    if (!destination) {
      setUrlError('Enter a valid http(s) URL (up to 2048 characters)')
      return
    }
    setUrlError(null)
    const bh = backHalf.trim()
    const bhError = validateBackHalf(bh)
    if (bhError) {
      setBackHalfError(bhError)
      return
    }
    let expiresIso: string | undefined
    if (expOn && expiresAt) {
      const d = new Date(expiresAt)
      if (Number.isNaN(d.getTime()) || d < minExpiration()) {
        setExpError('Pick a date at least 5 minutes from now')
        return
      }
      expiresIso = d.toISOString()
    }
    setExpError(null)
    const targetPage = pageOn ? pages.find((p) => String(p.id) === pageId) : undefined
    const blockTitle = buttonTitle.trim()
    if (targetPage && (blockTitle.length < 1 || blockTitle.length > BUTTON_TITLE_MAX)) {
      setButtonTitleError(`Enter a button title (1–${BUTTON_TITLE_MAX} characters)`)
      return
    }
    setButtonTitleError(null)

    const input: CreateLinkInput = {
      original_url: destination,
      custom_code: bh || undefined,
      title: title.trim() || undefined,
      domain_id: domainIdForCreate(domainChoice, groupDefaultDomainId),
      tags: tags.length ? tags : undefined,
      expires_at: expiresIso,
    }
    if (utmOn) {
      const cleaned = cleanUtm(utm)
      if (cleaned) input.utm = cleaned
    }
    if (routingOn) {
      const routing = toRoutingRuleInputs(rules)
      if (routing.length) input.routing_rules = routing
    }

    try {
      const link = await createLink.mutateAsync(input)
      if (input.utm) rememberUtm(input.utm)
      if (qrOn) {
        try {
          await createQr.mutateAsync({
            link_id: link.id,
            render_customizations: { ...DEFAULT_QR_DESIGN, color: qrColor, logo: qrLogo },
          })
        } catch {
          toast.error("Link created, but the QR Code couldn't be created")
        }
      }
      if (targetPage) {
        try {
          await addBlock.mutateAsync({
            pageId: targetPage.id,
            input: { type: 'link', link_id: link.id, content: { title: blockTitle, description: '', thumbnail_url: '' } },
          })
        } catch {
          toast.error("Link created, but it couldn't be added to the page")
        }
      }
      try {
        await navigator.clipboard.writeText(link.short_url)
        toast.success('Link created and copied')
      } catch {
        toast.success('Link created')
      }
      router.push('/dashboard/links/' + link.id)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create link'
      if (err instanceof ApiError && err.status === 409) setBackHalfError(message)
      toast.error(message)
    }
  }

  return (
    <form onSubmit={submit} noValidate data-mode={mode} className="mx-auto w-full max-w-[720px]">
      <div className="space-y-4 pb-24">
        <Section title="Link details">
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-url`}>Destination URL</Label>
            <Input
              id={`${id}-url`}
              type="url"
              required
              placeholder="https://example.com/my-long-url"
              value={url}
              maxLength={2048}
              aria-invalid={urlError ? true : undefined}
              onChange={(e) => {
                setUrl(e.target.value)
                if (urlError) setUrlError(null)
              }}
            />
            {urlError ? (
              <p className="text-sm text-destructive">{urlError}</p>
            ) : (
              <p className="text-sm text-muted-foreground">Hit Enter to create</p>
            )}
          </div>
          <ShortLinkField
            domains={domains}
            domainId={domainId}
            onDomainChange={setDomainChoice}
            backHalf={backHalf}
            onBackHalfChange={(v) => {
              setBackHalf(v)
              setBackHalfError(null)
            }}
            error={backHalfError}
          />
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-title`}>Title (optional)</Label>
            <Input id={`${id}-title`} value={title} maxLength={255} onChange={(e) => setTitle(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label>Tags</Label>
            <TagPicker value={tags} onChange={setTags} suggestions={tagSuggestions ?? []} />
          </div>
        </Section>

        <Section title="Sharing options">
          <SwitchRow label="Generate a QR Code" checked={qrOn} onCheckedChange={setQrOn} />
          {qrOn && (
            <div className="space-y-4 pl-12">
              <div className="space-y-2">
                <div className="text-sm font-semibold text-foreground">Code color</div>
                <div className="flex flex-wrap items-center gap-2">
                  {QR_COLORS.map((c) => (
                    <button
                      key={c}
                      type="button"
                      aria-label={`Color ${c}`}
                      aria-pressed={qrColor === c}
                      onClick={() => {
                        setQrColor(c)
                        setQrHex(c)
                      }}
                      className={cn(
                        'size-8 rounded-full border-2',
                        qrColor === c ? 'border-primary ring-2 ring-primary/30' : 'border-transparent',
                      )}
                      style={{ backgroundColor: c }}
                    />
                  ))}
                  <Input
                    aria-label="Custom color hex"
                    className="w-28"
                    value={qrHex}
                    maxLength={7}
                    onChange={(e) => {
                      const v = e.target.value
                      setQrHex(v)
                      if (HEX_RE.test(v)) setQrColor(v.toUpperCase())
                    }}
                  />
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor={`${id}-logo`}>Logo</Label>
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
                {upload.isPending ? (
                  <p className="text-sm text-muted-foreground">Uploading…</p>
                ) : (
                  qrLogoName && <p className="text-sm text-muted-foreground">{qrLogoName}</p>
                )}
              </div>
              <p className="text-sm text-muted-foreground">More customizations are available after creating.</p>
            </div>
          )}
          <SwitchRow label="Add to a page" checked={pageOn} onCheckedChange={setPageOn} />
          {pageOn && (
            <div className="space-y-4 pl-12">
              {pages.length === 0 ? (
                <NoPagesYet />
              ) : (
                <>
                  <div className="space-y-1.5">
                    <Label htmlFor={`${id}-page`}>Page</Label>
                    <PageSelect id={`${id}-page`} pages={pages} value={pageId} onChange={setPageChoice} />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor={`${id}-button-title`}>Button title</Label>
                    <Input
                      id={`${id}-button-title`}
                      value={buttonTitle}
                      maxLength={BUTTON_TITLE_MAX}
                      aria-invalid={buttonTitleError ? true : undefined}
                      onChange={(e) => {
                        setButtonTitleEdit(e.target.value)
                        setButtonTitleError(null)
                      }}
                    />
                    {buttonTitleError && <p className="text-sm text-destructive">{buttonTitleError}</p>}
                  </div>
                </>
              )}
            </div>
          )}
        </Section>

        <Section title="Advanced settings">
          <SwitchRow label="UTM parameters" checked={utmOn} onCheckedChange={setUtmOn} />
          {utmOn && <UtmBuilder value={utm} onChange={setUtm} destination={normalizeDestination(url) ?? url} />}
          <SwitchRow label="Link expiration" checked={expOn} onCheckedChange={setExpOn} />
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
          <SwitchRow label="Dynamic routing" checked={routingOn} onCheckedChange={setRoutingOn} />
          {routingOn && <RoutingRulesEditor value={rules} onChange={setRules} />}
        </Section>
      </div>

      <div className="sticky bottom-0 -mx-4 border-t border-border bg-background px-4 py-3">
        <div className="mx-auto flex max-w-[720px] justify-end gap-3">
          <Button type="button" variant="secondary" asChild>
            <Link href="/dashboard/links">Cancel</Link>
          </Button>
          <Button
            type="submit"
            disabled={createLink.isPending || createQr.isPending || addBlock.isPending || upload.isPending}
          >
            {createLink.isPending || createQr.isPending || addBlock.isPending ? 'Creating…' : 'Create your link'}
          </Button>
        </div>
      </div>
    </form>
  )
}
