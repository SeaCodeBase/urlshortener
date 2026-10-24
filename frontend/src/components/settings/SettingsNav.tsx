'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cn } from '@/lib/utils'

const BASE = '/dashboard/settings'

export const SETTINGS_NAV: { heading: string; items: { label: string; href: string }[] }[] = [
  { heading: 'Personal settings', items: [{ label: 'Profile', href: `${BASE}/profile` }] },
  {
    heading: 'Account settings',
    items: [
      { label: 'Account details', href: `${BASE}/account` },
      { label: 'Groups', href: `${BASE}/groups` },
      { label: 'Webhooks', href: `${BASE}/webhooks` },
      { label: 'Bulk upload', href: `${BASE}/bulk-upload` },
      { label: 'Activity log', href: `${BASE}/activity-log` },
    ],
  },
  { heading: 'Developer settings', items: [{ label: 'API', href: `${BASE}/api` }] },
]

function isActive(href: string, pathname: string) {
  return pathname === href || pathname.startsWith(`${href}/`)
}

/** Left sub-nav of the settings area (spec §5.11). */
export function SettingsNav() {
  const pathname = usePathname() ?? ''
  return (
    <nav aria-label="Settings" className="flex flex-col gap-6">
      {SETTINGS_NAV.map((group) => (
        <div key={group.heading}>
          <h2 className="mb-2 px-3 text-xs font-semibold tracking-wide text-muted-foreground">{group.heading}</h2>
          <ul className="flex flex-col gap-1">
            {group.items.map(({ label, href }) => {
              const active = isActive(href, pathname)
              return (
                <li key={href}>
                  <Link
                    href={href}
                    aria-current={active ? 'page' : undefined}
                    className={cn(
                      'flex h-9 items-center rounded-[4px] px-3 text-sm font-medium text-foreground transition-colors hover:bg-sidebar-active-bg/60',
                      active && 'bg-sidebar-active-bg text-sidebar-active-fg',
                    )}
                  >
                    {label}
                  </Link>
                </li>
              )
            })}
          </ul>
        </div>
      ))}
    </nav>
  )
}
