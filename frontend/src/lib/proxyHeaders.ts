/**
 * Request headers the Next.js API proxy forwards to the backend. User-Agent and
 * Referer feed public page-view tracking (device/browser, referrer).
 */
export const PROXY_REQUEST_HEADERS = ['Authorization', 'Content-Type', 'Accept', 'X-Group-ID', 'User-Agent', 'Referer'] as const

/** Backend response headers the proxy passes back (CSV export needs both). */
export const PROXY_RESPONSE_HEADERS = ['Content-Type', 'Content-Disposition'] as const

function pick(source: Headers, names: readonly string[]): Headers {
  const out = new Headers()
  for (const name of names) {
    const value = source.get(name)
    if (value != null) out.set(name, value)
  }
  if (!out.has('Content-Type')) out.set('Content-Type', 'application/json')
  return out
}

/**
 * Allowlisted request headers for the backend call; Content-Type defaults to JSON.
 * The visitor's IP goes along as X-Forwarded-For (from the incoming
 * X-Forwarded-For, else X-Real-IP) so the backend's ClientIP sees the visitor,
 * not the proxy.
 */
export function buildProxyRequestHeaders(incoming: Headers): Headers {
  const out = pick(incoming, PROXY_REQUEST_HEADERS)
  const ip = incoming.get('X-Forwarded-For') ?? incoming.get('X-Real-IP')
  if (ip) out.set('X-Forwarded-For', ip)
  return out
}

/** Allowlisted backend response headers for the client; Content-Type defaults to JSON. */
export function buildProxyResponseHeaders(upstream: Headers): Headers {
  return pick(upstream, PROXY_RESPONSE_HEADERS)
}
