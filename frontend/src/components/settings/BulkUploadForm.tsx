'use client'

import { useId, useRef, useState, type FormEvent } from 'react'
import { Download, Upload } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ApiError } from '@/lib/api'
import { useUploadBulkCsv } from '@/lib/hooks'
import { downloadBulkTemplate } from './bulkTemplate'

/** Server limit (2 MiB); checked client-side to fail fast. */
export const BULK_MAX_BYTES = 2 * 1024 * 1024

function validate(file: File): string | null {
  if (!file.name.toLowerCase().endsWith('.csv')) return 'Choose a .csv file'
  if (file.size > BULK_MAX_BYTES) return 'File is larger than 2 MB'
  return null
}

/** Card with the template download and the CSV upload form. */
export function BulkUploadForm() {
  const id = useId()
  const upload = useUploadBulkCsv()
  const inputRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [error, setError] = useState<string | null>(null)

  function onChoose(chosen: File | undefined) {
    if (!chosen) {
      setFile(null)
      setError(null)
      return
    }
    const problem = validate(chosen)
    setError(problem)
    setFile(problem ? null : chosen)
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!file) return
    setError(null)
    try {
      await upload.mutateAsync(file)
      toast.success('Upload queued')
      setFile(null)
      if (inputRef.current) inputRef.current.value = ''
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't upload the file")
    }
  }

  return (
    <section className="rounded-xl bg-card p-6">
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <p className="max-w-prose text-sm text-muted-foreground">
          Upload a CSV with the columns long_url, title, custom_back_half, tags and domain. Tags are separated by |. Up
          to 1000 rows, 2 MB. Save as CSV UTF-8.
        </p>
        <Button type="button" variant="secondary" className="shrink-0" onClick={downloadBulkTemplate}>
          <Download className="size-4" />
          Download template
        </Button>
      </div>
      <form onSubmit={onSubmit} className="flex flex-col gap-2">
        <Label htmlFor={`${id}-file`}>CSV file</Label>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <Input
            ref={inputRef}
            id={`${id}-file`}
            type="file"
            accept=".csv,text/csv"
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? `${id}-error` : undefined}
            onChange={(e) => onChoose(e.target.files?.[0])}
            className="sm:max-w-sm"
          />
          <Button type="submit" disabled={!file || upload.isPending}>
            <Upload className="size-4" />
            {upload.isPending ? 'Uploading...' : 'Upload'}
          </Button>
        </div>
        {error && (
          <p id={`${id}-error`} className="text-sm text-destructive">
            {error}
          </p>
        )}
      </form>
    </section>
  )
}
