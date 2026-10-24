'use client'

import { useState } from 'react'
import { useParams } from 'next/navigation'
import { toast } from 'sonner'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { SettingsSection } from '@/components/settings/SettingsSection'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { ApiError } from '@/lib/api'
import { useDomains, useGroups, useMeta, useUpdateGroup, useUpdateMe } from '@/lib/hooks'
import { useAuthStore } from '@/stores/auth'
import type { Group, UpdateGroupInput } from '@/types'

const BASE_DOMAIN = 'base'

function hostOf(url: string | undefined): string {
  if (!url) return 'Default domain'
  try {
    return new URL(url).host
  } catch {
    return url
  }
}

function GroupForm({ group }: { group: Group }) {
  const { data: domains } = useDomains()
  const { data: meta } = useMeta()
  const updateGroup = useUpdateGroup()
  const updateMe = useUpdateMe()
  const defaultGroupId = useAuthStore((s) => s.user?.default_group_id ?? null)

  const [name, setName] = useState(group.name)
  const [domain, setDomain] = useState(group.default_domain_id != null ? String(group.default_domain_id) : BASE_DOMAIN)
  const [domainError, setDomainError] = useState<string | null>(null)

  const available = (domains ?? []).filter((d) => d.group_id == null || d.group_id === group.id)
  const isDefaultGroup = defaultGroupId === group.id

  async function save(e: React.FormEvent) {
    e.preventDefault()
    setDomainError(null)
    const data: UpdateGroupInput = { name: name.trim() }
    if (domain === BASE_DOMAIN) data.clear_default_domain = true
    else data.default_domain_id = Number(domain)
    try {
      await updateGroup.mutateAsync({ id: group.id, data })
      toast.success('Group updated')
    } catch (err) {
      if (err instanceof ApiError && err.status === 400 && /domain/i.test(err.message)) {
        setDomainError('This domain is not verified yet')
      } else {
        toast.error(err instanceof Error ? err.message : 'Failed to update group')
      }
    }
  }

  async function makeDefault(checked: boolean) {
    // The default group can't be unset, only replaced by another group.
    if (!checked) return
    try {
      await updateMe.mutateAsync({ default_group_id: group.id })
      toast.success('Default group updated')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update default group')
    }
  }

  return (
    <SettingsSection title="Group details">
      <form onSubmit={save} className="grid gap-5">
        <div className="grid gap-1.5">
          <Label htmlFor="group-name">Group name</Label>
          <Input id="group-name" value={name} onChange={(e) => setName(e.target.value)} maxLength={100} />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="group-domain">Default domain</Label>
          <Select
            value={domain}
            onValueChange={(v) => {
              setDomain(v)
              setDomainError(null)
            }}
          >
            <SelectTrigger id="group-domain" className="w-full" aria-invalid={!!domainError}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={BASE_DOMAIN}>{hostOf(meta?.base_url)} (default)</SelectItem>
              {available.map((d) => {
                const verified = d.verification_status === 'ok'
                return (
                  <SelectItem key={d.id} value={String(d.id)} disabled={!verified}>
                    {verified ? d.domain : `${d.domain} (pending verification)`}
                  </SelectItem>
                )
              })}
            </SelectContent>
          </Select>
          {domainError && <p className="text-sm text-destructive">{domainError}</p>}
        </div>
        <div className="flex items-center justify-between gap-4">
          <Label htmlFor="group-default">Set as my default group</Label>
          <Switch
            id="group-default"
            checked={isDefaultGroup}
            disabled={isDefaultGroup || updateMe.isPending}
            onCheckedChange={makeDefault}
          />
        </div>
        <div>
          <Button type="submit" disabled={!name.trim() || updateGroup.isPending}>
            Save
          </Button>
        </div>
      </form>
    </SettingsSection>
  )
}

export default function GroupDetailPage() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)
  const { data: groups, isLoading } = useGroups()
  const group = groups?.find((g) => g.id === id)

  return (
    <>
      <PageHeader title={group?.name ?? 'Group'} backHref="/dashboard/settings/groups" />
      {isLoading ? (
        <Skeleton className="h-64 w-full rounded-xl" />
      ) : group ? (
        <GroupForm key={`${group.id}:${group.updated_at}`} group={group} />
      ) : (
        <EmptyState title="Group not found" description="It may have been removed or belong to another account." />
      )}
    </>
  )
}
