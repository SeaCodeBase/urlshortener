'use client'

import { useId, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ApiError } from '@/lib/api'

export interface DashboardNameDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  mode: 'create' | 'rename'
  /** Prefilled name (rename). */
  initialName?: string
  /** Receives the trimmed name; the dialog closes when it resolves and toasts when it rejects. */
  onSubmit: (name: string) => Promise<void>
}

const MAX_NAME = 255

const COPY = {
  create: { title: 'Create dashboard', submit: 'Create dashboard', error: "Couldn't create the dashboard" },
  rename: { title: 'Rename dashboard', submit: 'Save', error: "Couldn't rename the dashboard" },
} as const

/** Name a new dashboard or rename one: "Dashboard name" input, Cancel / submit. */
export function DashboardNameDialog({ open, onOpenChange, mode, initialName, onSubmit }: DashboardNameDialogProps) {
  const [saving, setSaving] = useState(false)
  return (
    <Dialog open={open} onOpenChange={(o) => !saving && onOpenChange(o)}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">{COPY[mode].title}</DialogTitle>
        </DialogHeader>
        {/* Mounted only while open so the draft resets each time. */}
        {open && (
          <NameForm
            mode={mode}
            initialName={initialName ?? ''}
            saving={saving}
            onCancel={() => onOpenChange(false)}
            onSubmit={async (name) => {
              setSaving(true)
              try {
                await onSubmit(name)
                onOpenChange(false)
              } catch (err) {
                toast.error(err instanceof ApiError ? err.message : COPY[mode].error)
              } finally {
                setSaving(false)
              }
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function NameForm({
  mode,
  initialName,
  saving,
  onCancel,
  onSubmit,
}: {
  mode: 'create' | 'rename'
  initialName: string
  saving: boolean
  onCancel: () => void
  onSubmit: (name: string) => void
}) {
  const id = useId()
  const [name, setName] = useState(initialName)
  const trimmed = name.trim()
  const valid = trimmed.length > 0 && trimmed.length <= MAX_NAME

  function submit(e: FormEvent) {
    e.preventDefault()
    if (valid && !saving) onSubmit(trimmed)
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <Label htmlFor={id}>Dashboard name</Label>
        <Input id={id} value={name} onChange={(e) => setName(e.target.value)} maxLength={MAX_NAME} autoFocus />
      </div>
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onCancel} disabled={saving}>
          Cancel
        </Button>
        <Button type="submit" disabled={saving || !valid}>
          {COPY[mode].submit}
        </Button>
      </DialogFooter>
    </form>
  )
}
