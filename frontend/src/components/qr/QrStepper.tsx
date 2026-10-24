import { Check } from 'lucide-react'
import { cn } from '@/lib/utils'

const STEPS = ['Configure code', 'Customize design'] as const

/** "Configure code → Customize design" progress for the QR create flow; `step` is 1 or 2. */
export function QrStepper({ step }: { step: 1 | 2 }) {
  return (
    <ol className="flex items-center gap-3 text-sm font-semibold" aria-label="Progress">
      {STEPS.map((label, i) => {
        const n = i + 1
        const done = n < step
        const current = n === step
        return (
          <li key={label} className="flex items-center gap-3" aria-current={current ? 'step' : undefined}>
            {i > 0 && (
              <span className="text-muted-foreground" aria-hidden>
                →
              </span>
            )}
            <span
              className={cn(
                'flex size-6 items-center justify-center rounded-full text-xs',
                done || current ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground',
              )}
            >
              {done ? <Check className="size-3.5" /> : n}
            </span>
            <span className={current ? 'text-foreground' : 'text-muted-foreground'}>{label}</span>
          </li>
        )
      })}
    </ol>
  )
}
