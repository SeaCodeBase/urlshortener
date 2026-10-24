'use client'

import { useState, type ReactNode } from 'react'
import Link from 'next/link'
import { CheckCircle2, Circle, X } from 'lucide-react'
import { toast } from 'sonner'
import { ShareDialog } from '@/components/links/ShareDialog'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { useLinks, useQrCodes, useUpdateMe } from '@/lib/hooks'
import { useAuthStore } from '@/stores/auth'

const CLICK_GOAL = 10

function ProgressRing({ pct }: { pct: number }) {
  const r = 18
  const c = 2 * Math.PI * r
  return (
    <div className="relative size-12 shrink-0" role="img" aria-label={`${pct}% complete`}>
      <svg viewBox="0 0 44 44" className="size-12 -rotate-90">
        <circle cx="22" cy="22" r={r} fill="none" strokeWidth="4" className="stroke-muted" />
        <circle
          cx="22"
          cy="22"
          r={r}
          fill="none"
          strokeWidth="4"
          strokeLinecap="round"
          className="stroke-primary transition-all"
          strokeDasharray={c}
          strokeDashoffset={c - (pct / 100) * c}
        />
      </svg>
      <span className="absolute inset-0 flex items-center justify-center text-xs font-semibold text-foreground">
        {pct}%
      </span>
    </div>
  )
}

function Item({ done, title, children }: { done: boolean; title: string; children?: ReactNode }) {
  const Icon = done ? CheckCircle2 : Circle
  return (
    <li className="flex gap-3 py-3">
      <Icon className={done ? 'mt-0.5 size-5 shrink-0 text-success' : 'mt-0.5 size-5 shrink-0 text-muted-foreground'} />
      <div className="min-w-0 flex-1">
        <div className={done ? 'font-medium text-muted-foreground line-through' : 'font-medium text-foreground'}>
          {title}
        </div>
        {children && <div className="mt-1.5 space-y-2 text-sm">{children}</div>}
      </div>
    </li>
  )
}

/** "Hi {name}, get started" checklist; dismissable per user. */
export function GettingStarted() {
  const user = useAuthStore((s) => s.user)
  const updateMe = useUpdateMe()
  const { data } = useLinks({ limit: 100 })
  const { data: qrData } = useQrCodes({ limit: 100, archived: 'both' })
  const [shareOpen, setShareOpen] = useState(false)

  if (!user || user.home_checklist_dismissed) return null

  const firstPage = data?.pages[0]
  const total = firstPage?.total ?? 0
  const newest = firstPage?.links[0]
  const linkClicks = firstPage?.links.reduce((sum, l) => sum + l.total_clicks, 0) ?? 0
  const qrFirstPage = qrData?.pages[0]
  const scans = qrFirstPage?.qr_codes.reduce((sum, q) => sum + q.total_scans, 0) ?? 0
  const clicks = linkClicks + scans
  const name = user.display_name?.trim() || user.email.split('@')[0]

  const hasQr = (qrFirstPage?.total ?? 0) > 0
  const steps = [total >= 1, clicks >= CLICK_GOAL, hasQr, false]
  const pct = Math.round((steps.filter(Boolean).length / steps.length) * 100)

  function dismiss() {
    updateMe.mutate(
      { home_checklist_dismissed: true },
      { onError: (err) => toast.error(err instanceof Error ? err.message : "Couldn't dismiss the checklist") },
    )
  }

  return (
    <div className="rounded-xl border border-border bg-card p-6">
      <div className="flex items-start justify-between gap-4">
        <h2 className="text-lg font-semibold text-foreground">Hi {name}, get started</h2>
        <div className="flex items-center gap-2">
          <ProgressRing pct={pct} />
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Dismiss checklist"
            disabled={updateMe.isPending}
            onClick={dismiss}
          >
            <X className="size-4" />
          </Button>
        </div>
      </div>
      <ul className="mt-2 divide-y divide-border">
        <Item done={steps[0]} title="Shorten a URL">
          {!steps[0] && (
            <Link href="/dashboard/links/create" className="font-semibold text-primary hover:underline">
              Let&apos;s make a link
            </Link>
          )}
        </Item>
        <Item done={steps[1]} title="Collect 10 clicks or scans">
          <Progress value={Math.min(100, (clicks / CLICK_GOAL) * 100)} aria-label="Clicks goal" />
          <div className="text-muted-foreground">
            {Math.min(clicks, CLICK_GOAL)}/{CLICK_GOAL} clicks to goal
          </div>
          <div className="flex gap-4">
            {newest && (
              <button type="button" onClick={() => setShareOpen(true)} className="font-semibold text-primary hover:underline">
                Share first link
              </button>
            )}
            <Link href="/dashboard/links" className="font-semibold text-primary hover:underline">
              View links
            </Link>
          </div>
        </Item>
        <Item done={steps[2]} title="Create a QR Code">
          {!steps[2] && (
            <Link href="/dashboard/qr-codes/create" className="font-semibold text-primary hover:underline">
            Create a QR Code
            </Link>
          )}
        </Item>
        <Item done={steps[3]} title="Create a page">
          <Link href="/dashboard/pages/new" className="font-semibold text-primary hover:underline">
            Create a page
          </Link>
        </Item>
      </ul>
      {newest && (
        <ShareDialog
          open={shareOpen}
          onOpenChange={setShareOpen}
          shortUrl={newest.short_url}
          title={newest.title}
        />
      )}
    </div>
  )
}
