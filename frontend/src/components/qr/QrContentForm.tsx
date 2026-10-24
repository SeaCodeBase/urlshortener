'use client'

import { useId } from 'react'
import { TagPicker } from '@/components/common/TagPicker'
import { ExpirationField } from '@/components/links/ExpirationField'
import { Section, SwitchRow } from '@/components/links/LinkForm'
import { RoutingRulesEditor, emptyRule, type RoutingRuleDraft } from '@/components/links/RoutingRulesEditor'
import {
  ShortLinkField,
  useDefaultHost,
  useGroupDefaultDomainId,
  usableDomains,
} from '@/components/links/ShortLinkField'
import { UtmBuilder } from '@/components/links/UtmBuilder'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatShortUrl } from '@/lib/format'
import { useActiveGroupId, useDomains, useTags } from '@/lib/hooks'
import { normalizeDestination } from '@/lib/url'
import type { Link, UTMParams } from '@/types'

const DEFAULT_DOMAIN = 'default'

/** Step 1 of "Create a new QR Code": what the code points to. */
export interface QrContentDraft {
  destination: string
  /** undefined until the user picks one (the group default is shown); null = service default. */
  domainChoice: number | null | undefined
  shareAsLink: boolean
  backHalf: string
  title: string
  tags: string[]
  utmOn: boolean
  utm: UTMParams
  expirationOn: boolean
  /** Local `YYYY-MM-DDTHH:mm`. */
  expiresAt: string
  routingOn: boolean
  rules: RoutingRuleDraft[]
}

export interface QrContentErrors {
  destination?: string | null
  backHalf?: string | null
  expiresAt?: string | null
}

export function emptyQrContentDraft(destination = ''): QrContentDraft {
  return {
    destination,
    domainChoice: undefined,
    shareAsLink: false,
    backHalf: '',
    title: '',
    tags: [],
    utmOn: false,
    utm: {},
    expirationOn: false,
    expiresAt: '',
    routingOn: false,
    rules: [emptyRule()],
  }
}

/** "Code details" + "Advanced settings" of the QR create flow (step 1). */
export function QrContentForm({
  value,
  onChange,
  attachedLink,
  errors,
}: {
  value: QrContentDraft
  onChange: (next: QrContentDraft) => void
  /** Set when the code attaches to an existing link (`?link=`): its destination is fixed. */
  attachedLink?: Link
  errors: QrContentErrors
}) {
  const id = useId()
  const { data: domains } = useDomains()
  const { data: tagSuggestions } = useTags('qr')
  const activeGroupId = useActiveGroupId()
  const usable = usableDomains(domains, activeGroupId)
  const groupDefaultDomainId = useGroupDefaultDomainId(usable)
  const defaultHost = useDefaultHost()
  const domainId = value.domainChoice === undefined ? groupDefaultDomainId : value.domainChoice

  const set = (patch: Partial<QrContentDraft>) => onChange({ ...value, ...patch })

  return (
    <div className="space-y-4">
      <Section title="Code details">
        {attachedLink ? (
          <div className="space-y-1 rounded-lg bg-muted px-4 py-3">
            <p className="text-sm text-foreground">
              This QR Code will be attached to{' '}
              <span className="font-semibold text-primary">{formatShortUrl(attachedLink.short_url)}</span>
            </p>
            <p className="text-sm break-all text-muted-foreground">{attachedLink.original_url}</p>
          </div>
        ) : (
          <>
            <div className="space-y-1.5">
              <Label htmlFor={`${id}-url`}>Destination URL</Label>
              <Input
                id={`${id}-url`}
                type="url"
                required
                placeholder="https://example.com/my-long-url"
                value={value.destination}
                maxLength={2048}
                aria-invalid={errors.destination ? true : undefined}
                onChange={(e) => set({ destination: e.target.value })}
              />
              {errors.destination && <p className="text-sm text-destructive">{errors.destination}</p>}
            </div>
            {!value.shareAsLink && (
              <div className="space-y-1.5 sm:w-56">
                <Label htmlFor={`${id}-domain`}>Domain</Label>
                <Select
                  value={domainId == null ? DEFAULT_DOMAIN : String(domainId)}
                  onValueChange={(v) => set({ domainChoice: v === DEFAULT_DOMAIN ? null : Number(v) })}
                >
                  <SelectTrigger id={`${id}-domain`} className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={DEFAULT_DOMAIN}>{defaultHost || 'Default domain'}</SelectItem>
                    {usable.map((d) => (
                      <SelectItem key={d.id} value={String(d.id)}>
                        {d.domain}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
            <SwitchRow
              label="Share as a short link"
              checked={value.shareAsLink}
              onCheckedChange={(shareAsLink) => set({ shareAsLink })}
            />
            {value.shareAsLink && (
              <ShortLinkField
                domains={domains ?? []}
                domainId={domainId}
                onDomainChange={(domainChoice) => set({ domainChoice })}
                backHalf={value.backHalf}
                onBackHalfChange={(backHalf) => set({ backHalf })}
                error={errors.backHalf}
              />
            )}
          </>
        )}
        <div className="space-y-1.5">
          <Label htmlFor={`${id}-title`}>Title (optional)</Label>
          <Input id={`${id}-title`} value={value.title} maxLength={255} onChange={(e) => set({ title: e.target.value })} />
        </div>
        <div className="space-y-1.5">
          <Label>Tags</Label>
          <TagPicker value={value.tags} onChange={(tags) => set({ tags })} suggestions={tagSuggestions ?? []} />
        </div>
      </Section>

      {!attachedLink && (
        <Section title="Advanced settings">
          <SwitchRow label="UTM parameters" checked={value.utmOn} onCheckedChange={(utmOn) => set({ utmOn })} />
          {value.utmOn && (
            <UtmBuilder
              value={value.utm}
              onChange={(utm) => set({ utm })}
              destination={normalizeDestination(value.destination) ?? value.destination}
            />
          )}
          <SwitchRow
            label="Code expiration"
            checked={value.expirationOn}
            onCheckedChange={(expirationOn) => set({ expirationOn })}
          />
          {value.expirationOn && (
            <ExpirationField value={value.expiresAt} onChange={(expiresAt) => set({ expiresAt })} error={errors.expiresAt} />
          )}
          <SwitchRow label="Dynamic routing" checked={value.routingOn} onCheckedChange={(routingOn) => set({ routingOn })} />
          {value.routingOn && <RoutingRulesEditor value={value.rules} onChange={(rules) => set({ rules })} />}
        </Section>
      )}
    </div>
  )
}
