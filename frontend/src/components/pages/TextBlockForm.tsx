'use client'

import { useId, useState, type FormEvent } from 'react'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { charCount, Field, FormActions, invalidProps, tooLong, type BlockFormProps, type FieldErrors } from './blockFormShared'

const HEADING_MAX = 120
const BODY_MAX = 1000

/** Text block: heading and/or body. */
export function TextBlockForm({ block, submitting, onSubmit, onCancel }: BlockFormProps<'text'>) {
  const id = useId()
  const [heading, setHeading] = useState(block?.content.heading ?? '')
  const [body, setBody] = useState(block?.content.body ?? '')
  const [errors, setErrors] = useState<FieldErrors<'heading' | 'body' | 'form'>>({})

  function submit(e: FormEvent) {
    e.preventDefault()
    const h = heading.trim()
    const b = body.trim()
    const next: typeof errors = {}
    if (!h && !b) next.form = 'Add a heading or body'
    if (charCount(h) > HEADING_MAX) next.heading = tooLong('Heading', HEADING_MAX)
    if (charCount(b) > BODY_MAX) next.body = tooLong('Body', BODY_MAX)
    setErrors(next)
    if (Object.keys(next).length) return
    onSubmit({ content: { heading: h, body: b } })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-1 flex-col gap-4">
      <Field id={`${id}-heading`} label="Heading" error={errors.heading}>
        <Input
          id={`${id}-heading`}
          value={heading}
          onChange={(e) => setHeading(e.target.value)}
          {...invalidProps(`${id}-heading`, errors.heading)}
        />
      </Field>
      <Field id={`${id}-body`} label="Body" error={errors.body} hint={`${body.length}/${BODY_MAX}`}>
        <Textarea
          id={`${id}-body`}
          rows={6}
          value={body}
          onChange={(e) => setBody(e.target.value)}
          {...invalidProps(`${id}-body`, errors.body)}
        />
      </Field>
      {errors.form && <p className="text-sm text-destructive">{errors.form}</p>}
      <FormActions submitting={submitting} onCancel={onCancel} />
    </form>
  )
}
