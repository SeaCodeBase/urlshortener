import type { ReactNode } from 'react'

/** Centred bold title (20px) + grey description (16px) + optional action (spec §3 "Other"). */
export function EmptyState({
  title,
  description,
  action,
}: {
  title: string
  description?: ReactNode
  action?: ReactNode
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded-xl bg-card px-8 py-16 text-center">
      <h2 className="text-xl font-bold text-foreground">{title}</h2>
      {description && <p className="mt-2 max-w-md text-base text-muted-foreground">{description}</p>}
      {action && <div className="mt-6">{action}</div>}
    </div>
  )
}
