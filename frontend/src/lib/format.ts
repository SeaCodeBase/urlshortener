const numberFormatter = new Intl.NumberFormat("en-US");

/** `2026-10-24T09:37:00Z` → `Oct 24, 2026` (browser time zone). */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

/** → `October 24, 2026 5:37 PM GMT+8`. Uses the browser time zone unless one is given. */
export function formatDateTime(iso: string, timeZone?: string): string {
  const d = new Date(iso);
  const date = d.toLocaleDateString("en-US", {
    month: "long",
    day: "numeric",
    year: "numeric",
    timeZone,
  });
  const time = d.toLocaleTimeString("en-US", {
    hour: "numeric",
    minute: "2-digit",
    timeZoneName: "short",
    timeZone,
  });
  return `${date} ${time}`;
}

/** `1240` → `1,240`. */
export function formatNumber(n: number): string {
  return numberFormatter.format(n);
}

/** Integer percentage of `part` in `total`; `0%` when total is 0. */
export function pct(part: number, total: number): string {
  if (!total) return "0%";
  return `${Math.round((part / total) * 100)}%`;
}

/** Strips the protocol: `https://bit.ly/abc` → `bit.ly/abc`. */
export function formatShortUrl(url: string): string {
  return url.replace(/^[a-z][a-z0-9+.-]*:\/\//i, "");
}

/** `https://example.com/a?b` → `example.com`; returns the input when it isn't a URL. */
export function hostOf(url: string): string {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}
