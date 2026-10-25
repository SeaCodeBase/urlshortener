'use client'

import { useState } from 'react'
import { ExternalLink } from 'lucide-react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { SettingsSection } from '@/components/settings/SettingsSection'
import { TokenCreateDialog } from '@/components/settings/TokenCreateDialog'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatDate } from '@/lib/format'
import { useRevokeToken, useTokens } from '@/lib/hooks'
import type { APIToken } from '@/types'

export default function ApiSettingsPage() {
  const { data: tokens, isLoading } = useTokens()
  const revoke = useRevokeToken()
  const [createOpen, setCreateOpen] = useState(false)
  const [target, setTarget] = useState<APIToken | null>(null)

  async function confirmRevoke() {
    if (!target) return
    try {
      await revoke.mutateAsync(target.id)
      toast.success('Token revoked')
      setTarget(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to revoke token')
    }
  }

  return (
    <>
      <PageHeader
        title="API"
        actions={
          <a
            href="https://github.com/SeaCodeBase/urlshortener#api"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline"
          >
            View API documentation
            <ExternalLink className="size-4" aria-hidden />
          </a>
        }
      />
      <SettingsSection
        title="Access tokens"
        description="Some integrations from other apps will ask you for an access token. You'll also need one when connecting through the API."
        actions={<Button onClick={() => setCreateOpen(true)}>Generate token</Button>}
      >
        {isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : !tokens?.length ? (
          // The section header already carries "Generate token", so the empty state points at it rather than repeating it.
          <EmptyState title="No tokens generated yet" description="Generate a token to connect other apps or call the API." />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Token</TableHead>
                <TableHead>Last used</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>
                  <span className="sr-only">Actions</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {tokens.map((t) => (
                <TableRow key={t.id}>
                  <TableCell className="font-medium">{t.name}</TableCell>
                  <TableCell className="font-mono text-sm">us_{t.prefix}…</TableCell>
                  <TableCell>{t.last_used_at ? formatDate(t.last_used_at) : 'Never'}</TableCell>
                  <TableCell>{formatDate(t.created_at)}</TableCell>
                  <TableCell className="text-right">
                    <Button variant="link" size="sm" className="text-destructive" onClick={() => setTarget(t)}>
                      Revoke
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </SettingsSection>
      <TokenCreateDialog open={createOpen} onOpenChange={setCreateOpen} />
      <ConfirmDialog
        open={!!target}
        onOpenChange={(o) => !o && setTarget(null)}
        title="Revoke token?"
        description="Apps using this token will stop working."
        confirmLabel="Revoke"
        destructive
        loading={revoke.isPending}
        onConfirm={confirmRevoke}
      />
    </>
  )
}
