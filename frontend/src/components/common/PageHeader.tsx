import type { ReactNode } from 'react'

/** Page title row: h1 32px/600 at 32px from the top, primary action on the right. */
export function PageHeader({ title, action }: { title: string; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 pt-8 pb-6">
      <h1 className="text-[32px] leading-10 font-semibold tracking-[0.32px] text-foreground">{title}</h1>
      {action}
    </div>
  )
}
