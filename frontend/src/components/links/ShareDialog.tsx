'use client'

import type { ReactNode } from 'react'
import { Mail, MessageCircle } from 'lucide-react'
import { CopyButton } from '@/components/common/CopyButton'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { formatShortUrl } from '@/lib/format'

function XIcon() {
  return (
    <svg viewBox="0 0 24 24" className="size-5" fill="currentColor" aria-hidden>
      <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
    </svg>
  )
}

function FacebookIcon() {
  return (
    <svg viewBox="0 0 24 24" className="size-5" fill="currentColor" aria-hidden>
      <path d="M24 12.07C24 5.41 18.63 0 12 0S0 5.4 0 12.07C0 18.1 4.39 23.1 10.13 24v-8.44H7.08v-3.49h3.04V9.41c0-3.02 1.8-4.7 4.54-4.7 1.31 0 2.68.24 2.68.24v2.97h-1.5c-1.5 0-1.96.93-1.96 1.89v2.26h3.32l-.53 3.5h-2.8V24C19.62 23.1 24 18.1 24 12.07" />
    </svg>
  )
}

function LinkedInIcon() {
  return (
    <svg viewBox="0 0 24 24" className="size-5" fill="currentColor" aria-hidden>
      <path d="M20.45 20.45h-3.56v-5.57c0-1.33-.02-3.04-1.85-3.04-1.85 0-2.14 1.45-2.14 2.94v5.67H9.35V9h3.41v1.56h.05c.48-.9 1.64-1.85 3.37-1.85 3.6 0 4.27 2.37 4.27 5.46v6.28zM5.34 7.43a2.06 2.06 0 1 1 0-4.13 2.06 2.06 0 0 1 0 4.13M7.12 20.45H3.56V9h3.56zM22.22 0H1.77C.79 0 0 .77 0 1.73v20.54C0 23.23.79 24 1.77 24h20.45c.98 0 1.78-.77 1.78-1.73V1.73C24 .77 23.2 0 22.22 0" />
    </svg>
  )
}

/** Share URLs for each network, in Bitly's order. */
export function shareTargets(shortUrl: string, title?: string): { label: string; href: string; icon: ReactNode }[] {
  const u = encodeURIComponent(shortUrl)
  return [
    { label: 'X', href: `https://twitter.com/intent/tweet?url=${u}`, icon: <XIcon /> },
    { label: 'Facebook', href: `https://www.facebook.com/sharer/sharer.php?u=${u}`, icon: <FacebookIcon /> },
    { label: 'LinkedIn', href: `https://www.linkedin.com/sharing/share-offsite/?url=${u}`, icon: <LinkedInIcon /> },
    { label: 'WhatsApp', href: `https://wa.me/?text=${u}`, icon: <MessageCircle className="size-5" /> },
    {
      label: 'Email',
      href: `mailto:?subject=${encodeURIComponent(title ?? '')}&body=${u}`,
      icon: <Mail className="size-5" />,
    },
  ]
}

/** "Share your link" dialog: short link + Copy, then network share buttons (spec §5.4). */
export function ShareDialog({
  open,
  onOpenChange,
  shortUrl,
  title,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  shortUrl: string
  title?: string
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">Share your link</DialogTitle>
          <DialogDescription className="sr-only">Copy the short link or share it on a network</DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-2 rounded border border-border bg-background py-1 pr-1 pl-3">
          <span className="min-w-0 flex-1 truncate text-base font-medium text-primary">{formatShortUrl(shortUrl)}</span>
          <CopyButton text={shortUrl} label="Copy" size="sm" />
        </div>
        <div className="flex flex-wrap justify-between gap-3">
          {shareTargets(shortUrl, title).map((t) => (
            <a
              key={t.label}
              href={t.href}
              target="_blank"
              rel="noopener noreferrer"
              className="flex w-16 flex-col items-center gap-1.5 text-xs font-medium text-foreground"
            >
              <span className="flex size-12 items-center justify-center rounded-full border border-border bg-white hover:bg-muted">
                {t.icon}
              </span>
              {t.label}
            </a>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  )
}
