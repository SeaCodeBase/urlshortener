'use client'

import { useState } from 'react'
import { Plus } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ApiError } from '@/lib/api'
import { useRoutingRules, useSetRoutingRules } from '@/lib/hooks'
import { normalizeDestination } from '@/lib/url'
import type { RoutingRule } from '@/types'
import {
  RoutingRulesEditor,
  describeRoutingRule,
  emptyRule,
  fromRoutingRule,
  toRoutingRuleInputs,
  type RoutingRuleDraft,
} from './RoutingRulesEditor'

/** "Dynamic routing" card on the details page: rule list + "Add rules" dialog (spec §5.4). */
export function DynamicRoutingCard({ linkId, initialRules = [] }: { linkId: number; initialRules?: RoutingRule[] }) {
  const { data } = useRoutingRules(linkId)
  const rules = data ?? initialRules
  const save = useSetRoutingRules()
  const [open, setOpen] = useState(false)
  const [drafts, setDrafts] = useState<RoutingRuleDraft[]>([])
  const [error, setError] = useState<string | null>(null)

  function openEditor() {
    setDrafts(rules.length ? rules.map(fromRoutingRule) : [emptyRule()])
    setError(null)
    setOpen(true)
  }

  function submit() {
    const inputs = toRoutingRuleInputs(drafts)
    const normalized = inputs.map((r) => ({ ...r, long_url: normalizeDestination(r.long_url) ?? '' }))
    if (normalized.some((r) => !r.long_url)) {
      setError('Each rule needs a valid destination URL')
      return
    }
    setError(null)
    save.mutate(
      { id: linkId, rules: normalized },
      {
        onSuccess: () => {
          toast.success('Routing rules saved')
          setOpen(false)
        },
        onError: (err) => {
          const message = err instanceof ApiError ? err.message : "Couldn't save the routing rules"
          setError(message)
          toast.error(message)
        },
      },
    )
  }

  return (
    <section className="rounded-xl bg-card pt-4 pr-4 pb-6 pl-8">
      <div className="mb-3 flex items-center justify-between gap-4">
        <h2 className="text-xl font-semibold text-foreground">Dynamic routing</h2>
        <Button type="button" variant="secondary" onClick={openEditor}>
          <Plus className="size-4" />
          Add rules
        </Button>
      </div>
      {rules.length ? (
        <ol className="space-y-2">
          {rules.map((r, i) => (
            <li key={r.id ?? i} className="flex min-w-0 gap-1 text-sm text-foreground">
              <span className="shrink-0 font-semibold">Rule {i + 1}:</span>
              <span className="shrink-0">{describeRoutingRule(r)}</span>
              <span className="shrink-0 text-muted-foreground">→</span>
              <a
                href={r.long_url}
                target="_blank"
                rel="noopener noreferrer"
                className="truncate font-medium text-primary hover:underline"
              >
                {r.long_url}
              </a>
            </li>
          ))}
        </ol>
      ) : (
        <p className="text-sm text-muted-foreground">
          Automatically route visitors to different destinations based on device, location, and more.
        </p>
      )}

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-[640px]">
          <DialogHeader>
            <DialogTitle className="text-xl font-semibold">Dynamic routing</DialogTitle>
            <DialogDescription>Send visitors to a different destination based on their device or location.</DialogDescription>
          </DialogHeader>
          <RoutingRulesEditor value={drafts} onChange={setDrafts} />
          {error && <p className="text-sm text-destructive">{error}</p>}
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="button" disabled={save.isPending} onClick={submit}>
              {save.isPending ? 'Saving…' : 'Save'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
