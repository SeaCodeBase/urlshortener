/** Adds `https://` when the scheme is missing; null when it still isn't an http(s) URL. */
export function normalizeDestination(raw: string): string | null {
  const v = raw.trim()
  if (!v) return null
  const withScheme = /^[a-z][a-z0-9+.-]*:\/\//i.test(v) ? v : `https://${v}`
  try {
    const u = new URL(withScheme)
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return null
    if (!u.hostname) return null
    return withScheme.length <= 2048 ? withScheme : null
  } catch {
    return null
  }
}
