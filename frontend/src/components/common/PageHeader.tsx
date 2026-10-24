import Link from 'next/link'
import { ArrowLeft } from 'lucide-react'
import type { ReactNode } from 'react'

/** Page title row: h1 32px/600 at 32px from the top, actions right-aligned on the same row. */
export function PageHeader({
  title,
  actions,
  action,
  backHref,
  description,
}: {
  title: ReactNode
  actions?: ReactNode
  /** @deprecated alias of `actions` (Task 12). */
  action?: ReactNode
  backHref?: string
  description?: ReactNode
}) {
  const right = actions ?? action
  return (
    <div className="pt-8 pb-6">
      <div className="flex items-center justify-between gap-4">
        <div className="flex min-w-0 items-center gap-3">
          {backHref && (
            <Link
              href={backHref}
              aria-label="Back"
              className="inline-flex size-8 shrink-0 items-center justify-center rounded text-foreground hover:bg-accent"
            >
              <ArrowLeft className="size-5" />
            </Link>
          )}
          <h1 className="truncate text-[32px] leading-10 font-semibold tracking-[0.32px] text-foreground">{title}</h1>
        </div>
        {right && <div className="flex shrink-0 items-center gap-2">{right}</div>}
      </div>
      {description && <p className="mt-2 text-base text-muted-foreground">{description}</p>}
    </div>
  )
}
