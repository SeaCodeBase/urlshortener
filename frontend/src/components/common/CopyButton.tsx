'use client'

import { useEffect, useRef, useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

/** Copies `text` to the clipboard, toasts, and shows a check icon for 2 s. */
export function CopyButton({
  text,
  label,
  size = label ? 'default' : 'icon',
  className,
}: {
  text: string
  label?: string
  size?: 'icon' | 'sm' | 'default'
  className?: string
}) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current)
  }, [])

  async function copy() {
    try {
      await navigator.clipboard.writeText(text)
      toast.success('Copied to clipboard')
      setCopied(true)
      if (timer.current) clearTimeout(timer.current)
      timer.current = setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error("Couldn't copy to clipboard")
    }
  }

  const Icon = copied ? Check : Copy
  const iconOnly = size === 'icon' && !label
  return (
    <Button
      type="button"
      variant={iconOnly ? 'ghost' : 'secondary'}
      size={iconOnly ? 'icon-sm' : size === 'icon' ? 'default' : size}
      aria-label={label ?? 'Copy'}
      onClick={copy}
      className={cn(iconOnly && 'size-7 p-1 text-foreground', className)}
    >
      <Icon className={cn('size-5', copied && 'text-success')} />
      {label && <span>{label}</span>}
    </Button>
  )
}
