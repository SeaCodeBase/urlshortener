'use client'

import { useId, useState, type FormEvent } from 'react'
import { Upload } from 'lucide-react'
import { toast } from 'sonner'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useUploadImage } from '@/lib/hooks'
import { cn } from '@/lib/utils'
import { charCount, Field, FormActions, invalidProps, isHttpUrl, tooLong, type BlockFormProps, type FieldErrors } from './blockFormShared'

const ALT_MAX = 120

/** http(s) URL or a path returned by the uploads endpoint. */
const isImageUrl = (v: string) => isHttpUrl(v) || /^\/uploads\/[^/\s]+$/.test(v)

/** Image block: URL (typed or uploaded) and alt text. */
export function ImageBlockForm({ block, submitting, onSubmit, onCancel }: BlockFormProps<'image'>) {
  const id = useId()
  const upload = useUploadImage()
  const [url, setUrl] = useState(block?.content.url ?? '')
  const [alt, setAlt] = useState(block?.content.alt ?? '')
  const [errors, setErrors] = useState<FieldErrors<'url' | 'alt'>>({})
  const trimmed = url.trim()

  async function uploadFile(file: File | undefined) {
    if (!file) return
    try {
      const res = await upload.mutateAsync(file)
      setUrl(res.url)
      setErrors((e) => ({ ...e, url: undefined }))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Couldn't upload the image")
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    const a = alt.trim()
    const next: typeof errors = {}
    if (!trimmed) next.url = 'Enter an image URL'
    else if (!isImageUrl(trimmed)) next.url = 'Enter a valid URL'
    if (charCount(a) > ALT_MAX) next.alt = tooLong('Alt text', ALT_MAX)
    setErrors(next)
    if (Object.keys(next).length) return
    onSubmit({ content: { url: trimmed, alt: a } })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-1 flex-col gap-4">
      <Field id={`${id}-url`} label="Image URL" error={errors.url}>
        <div className="flex gap-2">
          <Input
            id={`${id}-url`}
            placeholder="https://"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            {...invalidProps(`${id}-url`, errors.url)}
          />
          <Label
            htmlFor={`${id}-file`}
            className={cn(
              'inline-flex h-10 shrink-0 cursor-pointer items-center gap-2 rounded border border-border bg-white px-3 text-base font-medium hover:bg-muted dark:bg-card',
              upload.isPending && 'pointer-events-none opacity-50',
            )}
          >
            <Upload className="size-4" aria-hidden />
            Upload
          </Label>
          <input
            id={`${id}-file`}
            type="file"
            accept="image/png,image/jpeg,image/gif,image/webp"
            className="sr-only"
            disabled={upload.isPending}
            onChange={(e) => {
              void uploadFile(e.target.files?.[0])
              e.target.value = ''
            }}
          />
        </div>
      </Field>
      {isImageUrl(trimmed) && (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={trimmed} alt="" className="max-h-48 w-full rounded-lg border border-border object-contain" />
      )}
      <Field id={`${id}-alt`} label="Alt text" error={errors.alt} hint="Describes the image for screen readers.">
        <Input id={`${id}-alt`} value={alt} onChange={(e) => setAlt(e.target.value)} {...invalidProps(`${id}-alt`, errors.alt)} />
      </Field>
      <FormActions submitting={submitting || upload.isPending} onCancel={onCancel} />
    </form>
  )
}
