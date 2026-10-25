'use client'

import { useState } from 'react'
import { FileText, MoreHorizontal, Settings, Star, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { ApiError } from '@/lib/api'
import { formatDate } from '@/lib/format'
import { useDeleteDomain, useUpdateGroup, useVerifyDomain } from '@/lib/hooks'
import { useAuthStore } from '@/stores/auth'
import type { Domain, DomainVerificationStatus, Group } from '@/types'
import { DomainSettingsDialog } from './DomainSettingsDialog'

const STATUS: Record<DomainVerificationStatus, { label: string; className: string }> = {
  pending: { label: 'Pending', className: 'bg-muted text-muted-foreground' },
  ok: { label: 'Verified', className: 'bg-success/15 text-success' },
  error: { label: 'Error', className: 'bg-destructive/10 text-destructive' },
}

/** One custom domain: name, status, group, default chips and actions (spec §5.10). */
export function DomainRow({
  domain,
  groups,
  onShowInstructions,
}: {
  domain: Domain
  groups: Group[]
  onShowInstructions: (domain: Domain) => void
}) {
  const verify = useVerifyDomain()
  const del = useDeleteDomain()
  const updateGroup = useUpdateGroup()
  const activeGroupId = useAuthStore((s) => s.activeGroupId)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [confirmRemove, setConfirmRemove] = useState(false)

  const status = STATUS[domain.verification_status] ?? STATUS.pending
  const groupName = domain.group_id == null ? 'All groups' : (groups.find((g) => g.id === domain.group_id)?.name ?? 'Group')
  const defaultFor = groups.filter((g) => g.default_domain_id === domain.id)
  const isActiveDefault = groups.find((g) => g.id === activeGroupId)?.default_domain_id === domain.id
  const verified = domain.verification_status === 'ok'

  async function runVerify() {
    try {
      const updated = await verify.mutateAsync(domain.id)
      if (updated.verification_status === 'ok') toast.success('Domain verified')
      else toast.error(updated.verification_error || "We couldn't verify this domain yet")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Verification failed')
    }
  }

  async function setAsDefault() {
    if (activeGroupId == null) return
    try {
      await updateGroup.mutateAsync({ id: activeGroupId, data: { default_domain_id: domain.id } })
      toast.success('Default domain updated')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Couldn't update the default domain")
    }
  }

  async function remove() {
    try {
      await del.mutateAsync(domain.id)
      setConfirmRemove(false)
      toast.success('Domain removed')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to remove domain')
    }
  }

  return (
    <li className="flex flex-col gap-4 rounded-xl bg-card p-6 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0 space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="truncate font-mono text-base font-semibold text-foreground">{domain.domain}</h2>
          <Badge className={status.className} title={domain.verification_status === 'error' ? domain.verification_error ?? undefined : undefined}>
            {status.label}
          </Badge>
          {defaultFor.map((g) => (
            <Badge key={g.id} variant="outline">
              Default for {g.name}
            </Badge>
          ))}
        </div>
        {domain.verification_status === 'error' && domain.verification_error && (
          <p className="text-sm text-destructive">{domain.verification_error}</p>
        )}
        <p className="text-sm text-muted-foreground">
          {groupName} · Added {formatDate(domain.created_at)}
        </p>
      </div>

      <div className="flex shrink-0 items-center gap-2">
        {domain.verification_status !== 'ok' && (
          <Button variant="secondary" size="sm" onClick={runVerify} disabled={verify.isPending}>
            {verify.isPending ? 'Verifying...' : 'Verify now'}
          </Button>
        )}
        <Button variant="secondary" size="sm" onClick={() => setSettingsOpen(true)}>
          <Settings className="size-4" />
          Settings
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon-sm" aria-label={`More actions for ${domain.domain}`}>
              <MoreHorizontal className="size-5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => onShowInstructions(domain)}>
              <FileText className="size-4" />
              View DNS instructions
            </DropdownMenuItem>
            {!isActiveDefault &&
              (verified ? (
                <DropdownMenuItem onSelect={() => void setAsDefault()} disabled={updateGroup.isPending || activeGroupId == null}>
                  <Star className="size-4" />
                  Set as default for group
                </DropdownMenuItem>
              ) : (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <div>
                      <DropdownMenuItem disabled>
                        <Star className="size-4" />
                        Set as default for group
                      </DropdownMenuItem>
                    </div>
                  </TooltipTrigger>
                  <TooltipContent side="left">Verify the domain first</TooltipContent>
                </Tooltip>
              ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => setConfirmRemove(true)}>
              <Trash2 className="size-4" />
              Remove
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <DomainSettingsDialog domain={domain} open={settingsOpen} onOpenChange={setSettingsOpen} />
      <ConfirmDialog
        open={confirmRemove}
        onOpenChange={setConfirmRemove}
        title="Remove domain?"
        description="Links on this domain will stop working."
        confirmLabel="Remove"
        destructive
        loading={del.isPending}
        onConfirm={remove}
      />
    </li>
  )
}
