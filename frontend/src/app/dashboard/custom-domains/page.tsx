'use client'

import { useState } from 'react'
import { Check, Plus } from 'lucide-react'
import { PageHeader } from '@/components/common/PageHeader'
import { AddDomainDialog } from '@/components/domains/AddDomainDialog'
import { DomainRow } from '@/components/domains/DomainRow'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useDomains, useGroups } from '@/lib/hooks'
import type { Domain } from '@/types'

const BULLETS = [
  'Replace the default domain in your short links with your own',
  'Help people recognize and trust your links',
  'Keep every link on brand',
]

export default function CustomDomainsPage() {
  const { data: domains, isLoading, isError } = useDomains()
  const { data: groups } = useGroups()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [instructionsFor, setInstructionsFor] = useState<Domain | null>(null)

  const empty = !isLoading && !isError && (domains ?? []).length === 0

  function openAdd() {
    setInstructionsFor(null)
    setDialogOpen(true)
  }

  function openInstructions(domain: Domain) {
    setInstructionsFor(domain)
    setDialogOpen(true)
  }

  return (
    <div>
      <PageHeader
        title="Custom domains"
        actions={
          !empty && !isLoading ? (
            <Button onClick={openAdd}>
              <Plus className="size-5" />
              Add a domain
            </Button>
          ) : undefined
        }
      />

      {isLoading && (
        <div className="space-y-4">
          <Skeleton className="h-24 w-full rounded-xl" />
          <Skeleton className="h-24 w-full rounded-xl" />
        </div>
      )}

      {isError && (
        <p role="alert" className="rounded-xl bg-card p-6 text-destructive">
          Failed to load domains
        </p>
      )}

      {empty && (
        <section className="rounded-xl bg-card p-8 sm:p-12">
          <h2 className="text-2xl font-semibold text-foreground">Brand your links with a custom domain</h2>
          <ul className="mt-6 space-y-3">
            {BULLETS.map((b) => (
              <li key={b} className="flex items-start gap-3 text-base text-foreground">
                <Check className="mt-0.5 size-5 shrink-0 text-success" aria-hidden />
                {b}
              </li>
            ))}
          </ul>
          <Button className="mt-8" onClick={openAdd}>
            Add a domain you already own
          </Button>
        </section>
      )}

      {!isLoading && (domains ?? []).length > 0 && (
        <ul className="space-y-4">
          {(domains ?? []).map((d) => (
            <DomainRow key={d.id} domain={d} groups={groups ?? []} onShowInstructions={openInstructions} />
          ))}
        </ul>
      )}

      <AddDomainDialog open={dialogOpen} onOpenChange={setDialogOpen} initialDomain={instructionsFor} />
    </div>
  )
}
