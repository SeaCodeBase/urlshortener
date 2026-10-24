'use client'

import { useId } from 'react'
import { Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { RoutingRule, RoutingRuleInput } from '@/types'

export const MAX_ROUTING_RULES = 20

export type Device = 'any' | 'mobile' | 'tablet' | 'desktop'
export type OS = 'any' | 'ios' | 'android'

export interface RoutingRuleDraft {
  long_url: string
  device: Device
  os: OS
  /** Comma-separated ISO 3166-1 alpha-2 codes. */
  countries: string
  /** Countries are excluded instead of matched. */
  exclude: boolean
}

export const emptyRule = (): RoutingRuleDraft => ({ long_url: '', device: 'any', os: 'any', countries: '', exclude: false })

/** `"us, de ,,fr"` → `["US","DE","FR"]` (deduplicated). */
export function parseCountries(raw: string): string[] {
  return Array.from(new Set(raw.split(',').map((c) => c.trim().toUpperCase()).filter(Boolean)))
}

/** Editor drafts → API rules; fully empty rows are dropped. */
export function toRoutingRuleInputs(rules: RoutingRuleDraft[]): RoutingRuleInput[] {
  return rules
    .filter((r) => r.long_url.trim() || r.device !== 'any' || r.os !== 'any' || r.countries.trim())
    .map((r) => {
      const countries = parseCountries(r.countries)
      return {
        long_url: r.long_url.trim(),
        device_match: r.device === 'any' ? [] : [r.device],
        os_match: r.os === 'any' ? [] : [r.os],
        country_match: r.exclude ? [] : countries,
        country_exclude: r.exclude ? countries : [],
      }
    })
}

const DEVICE_VALUES: readonly Device[] = ['mobile', 'tablet', 'desktop']
const OS_VALUES: readonly OS[] = ['ios', 'android']

/** A saved rule → editor draft (the editor holds one device and one OS per rule). */
export function fromRoutingRule(rule: Pick<RoutingRule, 'long_url' | 'device_match' | 'os_match' | 'country_match' | 'country_exclude'>): RoutingRuleDraft {
  const device = (rule.device_match ?? []).find((d) => DEVICE_VALUES.includes(d as Device)) as Device | undefined
  const os = (rule.os_match ?? []).find((o) => OS_VALUES.includes(o as OS)) as OS | undefined
  const excluded = rule.country_exclude ?? []
  const exclude = excluded.length > 0
  return {
    long_url: rule.long_url,
    device: device ?? 'any',
    os: os ?? 'any',
    countries: (exclude ? excluded : rule.country_match ?? []).join(', '),
    exclude,
  }
}

const OS_LABELS: Record<string, string> = { ios: 'iOS', android: 'Android' }

/** `mobile · iOS · US`, `not US, CA`, or `any visitor` when the rule has no conditions. */
export function describeRoutingRule(rule: Pick<RoutingRule, 'device_match' | 'os_match' | 'country_match' | 'country_exclude'>): string {
  const parts = [
    ...(rule.device_match ?? []),
    ...(rule.os_match ?? []).map((o) => OS_LABELS[o] ?? o),
  ]
  if (rule.country_match?.length) parts.push(rule.country_match.join(', '))
  if (rule.country_exclude?.length) parts.push(`not ${rule.country_exclude.join(', ')}`)
  return parts.length ? parts.join(' · ') : 'any visitor'
}

const DEVICES: { value: Device; label: string }[] = [
  { value: 'any', label: 'Any device' },
  { value: 'mobile', label: 'Mobile' },
  { value: 'tablet', label: 'Tablet' },
  { value: 'desktop', label: 'Desktop' },
]
const OSES: { value: OS; label: string }[] = [
  { value: 'any', label: 'Any OS' },
  { value: 'ios', label: 'iOS' },
  { value: 'android', label: 'Android' },
]

/** Ordered dynamic-routing rules (first match wins), max 20. */
export function RoutingRulesEditor({
  value,
  onChange,
}: {
  value: RoutingRuleDraft[]
  onChange: (rules: RoutingRuleDraft[]) => void
}) {
  const id = useId()
  const update = (i: number, patch: Partial<RoutingRuleDraft>) =>
    onChange(value.map((r, j) => (j === i ? { ...r, ...patch } : r)))

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Rules apply in order, first match wins; all other traffic goes to the default destination.
      </p>
      {value.map((rule, i) => (
        <div key={i} className="space-y-3 rounded-lg border border-border p-4">
          <div className="flex items-center justify-between">
            <span className="text-sm font-semibold text-foreground">Rule {i + 1}</span>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={`Remove rule ${i + 1}`}
              onClick={() => onChange(value.filter((_, j) => j !== i))}
            >
              <X className="size-4" />
            </Button>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-url-${i}`}>Destination URL</Label>
            <Input
              id={`${id}-url-${i}`}
              placeholder="https://example.com/mobile"
              value={rule.long_url}
              onChange={(e) => update(i, { long_url: e.target.value })}
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor={`${id}-device-${i}`}>Device</Label>
              <Select value={rule.device} onValueChange={(v) => update(i, { device: v as Device })}>
                <SelectTrigger id={`${id}-device-${i}`} className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {DEVICES.map((d) => (
                    <SelectItem key={d.value} value={d.value}>
                      {d.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor={`${id}-os-${i}`}>OS</Label>
              <Select value={rule.os} onValueChange={(v) => update(i, { os: v as OS })}>
                <SelectTrigger id={`${id}-os-${i}`} className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {OSES.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-countries-${i}`}>Countries</Label>
            <Input
              id={`${id}-countries-${i}`}
              placeholder="US, CA, GB"
              value={rule.countries}
              onChange={(e) => update(i, { countries: e.target.value.toUpperCase() })}
            />
            <div className="flex items-center gap-2">
              <Checkbox
                id={`${id}-exclude-${i}`}
                checked={rule.exclude}
                onCheckedChange={(v) => update(i, { exclude: v === true })}
              />
              <Label htmlFor={`${id}-exclude-${i}`} className="font-normal">
                Exclude these countries
              </Label>
            </div>
          </div>
        </div>
      ))}
      <Button
        type="button"
        variant="link"
        className="h-auto px-0"
        disabled={value.length >= MAX_ROUTING_RULES}
        onClick={() => onChange([...value, emptyRule()])}
      >
        <Plus className="size-4" />
        Add rule
      </Button>
    </div>
  )
}
