'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { PageHeader } from '@/components/common/PageHeader'
import { ClaimUrlForm } from '@/components/pages/ClaimUrlForm'
import { TemplatePicker } from '@/components/pages/TemplatePicker'
import { ApiError } from '@/lib/api'
import { usePageMutations } from '@/lib/hooks'
import type { PageTemplate } from '@/types'

/** Create a page: claim the URL (step 1), then pick a template (step 2). */
export default function NewPagePage() {
  const router = useRouter()
  const { create } = usePageMutations()
  const [step, setStep] = useState<'claim' | 'template'>('claim')
  const [slug, setSlug] = useState('')
  const [conflictSlug, setConflictSlug] = useState<string | null>(null)
  const [pending, setPending] = useState<PageTemplate | null>(null)

  async function pick(template: PageTemplate) {
    setPending(template)
    try {
      const page = await create.mutateAsync({ slug, template })
      router.push(`/dashboard/pages/${page.id}`)
    } catch (err) {
      setPending(null)
      if (err instanceof ApiError && err.status === 409) {
        setConflictSlug(slug)
        setStep('claim')
      } else {
        toast.error(err instanceof ApiError ? err.message : "Couldn't create the page")
      }
    }
  }

  return (
    <div className="mx-auto flex max-w-4xl flex-col pb-12">
      <PageHeader title="Create a page" backHref="/dashboard/pages" />
      {step === 'claim' ? (
        <ClaimUrlForm
          initialSlug={slug}
          conflictSlug={conflictSlug}
          onSubmit={(s) => {
            setSlug(s)
            setStep('template')
          }}
        />
      ) : (
        <TemplatePicker slug={slug} pending={pending} onPick={(t) => void pick(t)} onBack={() => setStep('claim')} />
      )}
    </div>
  )
}
