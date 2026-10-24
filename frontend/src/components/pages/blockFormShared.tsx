'use client'

import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import type { BlockContentMap, BlockType, PageBlock } from '@/types'

/** What a block form hands back: content plus, for new link blocks, the link to use. */
export interface BlockFormValues<T extends BlockType> {
  content: BlockContentMap[T]
  link_id?: number
  new_link?: { original_url: string; title?: string }
}

export interface BlockFormProps<T extends BlockType> {
  /** The block being edited; undefined in create mode. */
  block?: Extract<PageBlock, { type: T }>
  submitting: boolean
  onSubmit: (values: BlockFormValues<T>) => void
  onCancel: () => void
}

export type FieldErrors<K extends string> = Partial<Record<K, string>>

/** Characters as the backend counts them (code points, not UTF-16 units). */
export const charCount = (s: string) => [...s].length

export const tooLong = (label: string, max: number) => `${label} must be ${max} characters or fewer`

/** An absolute http(s) URL with a host. */
export function isHttpUrl(v: string): boolean {
  try {
    const u = new URL(v)
    return (u.protocol === 'http:' || u.protocol === 'https:') && !!u.hostname
  } catch {
    return false
  }
}

/** Label, control and inline error. */
export function Field({
  id,
  label,
  error,
  hint,
  children,
}: {
  id: string
  label: string
  error?: string
  hint?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {hint && !error && <p className="text-xs text-muted-foreground">{hint}</p>}
      {error && (
        <p id={`${id}-error`} className="text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}

/** Sticky Save / Cancel row at the bottom of a block form. */
export function FormActions({ submitting, onCancel }: { submitting: boolean; onCancel: () => void }) {
  return (
    <div className="mt-auto flex justify-end gap-2 border-t border-border pt-4">
      <Button type="button" variant="secondary" onClick={onCancel}>
        Cancel
      </Button>
      <Button type="submit" disabled={submitting}>
        Save
      </Button>
    </div>
  )
}

/** Props for an input wired to a Field's error. */
export const invalidProps = (id: string, error?: string) =>
  error ? { 'aria-invalid': true as const, 'aria-describedby': `${id}-error` } : {}
