/** Request headers the Next.js API proxy forwards to the backend. */
export const PROXY_REQUEST_HEADERS = ['Authorization', 'Content-Type', 'Accept', 'X-Group-ID'] as const

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

/** Allowlisted request headers for the backend call; Content-Type defaults to JSON. */
export function buildProxyRequestHeaders(incoming: Headers): Headers {
  return pick(incoming, PROXY_REQUEST_HEADERS)
}

/** Allowlisted backend response headers for the client; Content-Type defaults to JSON. */
export function buildProxyResponseHeaders(upstream: Headers): Headers {
  return pick(upstream, PROXY_RESPONSE_HEADERS)
}
