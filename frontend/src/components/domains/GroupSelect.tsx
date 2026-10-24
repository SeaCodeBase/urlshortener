'use client'

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useGroups } from '@/lib/hooks'

export const ALL_GROUPS = 'all'

/** Select value ("all" or a group id) → API `group_id`. */
export function groupValueToId(value: string): number | null {
  return value === ALL_GROUPS ? null : Number(value)
}

export function groupIdToValue(id: number | null): string {
  return id == null ? ALL_GROUPS : String(id)
}

/** "All groups" + each of the user's groups. */
export function GroupSelect({ id, value, onChange }: { id?: string; value: string; onChange: (v: string) => void }) {
  const { data: groups } = useGroups()
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger id={id} className="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL_GROUPS}>All groups</SelectItem>
        {(groups ?? []).map((g) => (
          <SelectItem key={g.id} value={String(g.id)}>
            {g.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
