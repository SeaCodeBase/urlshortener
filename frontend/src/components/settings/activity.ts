import type { ActivityEntry } from '@/types'

const ENTITY_LABELS: Record<string, string> = {
  link: 'link',
  domain: 'domain',
  group: 'group',
  token: 'API token',
  api_token: 'API token',
  qr: 'QR Code',
  page: 'page',
  campaign: 'campaign',
  webhook: 'webhook',
  bulk: 'bulk upload',
}

const VERB_LABELS: Record<string, string> = {
  create: 'Created',
  update: 'Updated',
  delete: 'Deleted',
  hide: 'Hid',
  unhide: 'Unhid',
  verify: 'Verified',
  revoke: 'Revoked',
  rename: 'Renamed',
  deactivate: 'Deactivated',
  activate: 'Activated',
}

/** `link.create` → "Created link", `link.back_half` → "Changed back-half". */
export function humanizeAction(action: string): string {
  const [entity, verb = ''] = action.split('.', 2)
  if (verb === 'back_half') return 'Changed back-half'
  const noun = ENTITY_LABELS[entity] ?? entity.replace(/_/g, ' ')
  const past = VERB_LABELS[verb] ?? (verb ? verb.charAt(0).toUpperCase() + verb.slice(1).replace(/_/g, ' ') : '')
  return past ? `${past} ${noun}` : noun
}

/** Prefers a human-readable name from the metadata over `{type} #{id}`. */
export function activityTarget(entry: ActivityEntry): string {
  const m = entry.metadata ?? {}
  for (const key of ['short_code', 'name', 'domain']) {
    const v = m[key]
    if (typeof v === 'string' && v) return v
  }
  return `${entry.entity_type} #${entry.entity_id}`
}

/** Metadata as `key: value` strings; nested values are compact JSON. */
export function activityDetails(metadata: Record<string, unknown> | null | undefined): string[] {
  return Object.entries(metadata ?? {}).map(([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`)
}
