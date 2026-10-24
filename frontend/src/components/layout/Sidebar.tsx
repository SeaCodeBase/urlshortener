'use client'

import { Fragment } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  BarChart2,
  ChevronLeft,
  ChevronRight,
  FileText,
  Globe,
  Home,
  Link2,
  Megaphone,
  QrCode,
  Settings,
  type LucideIcon,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { CreateNewMenu, useCreateShortcuts } from './CreateNewMenu'
import { useSidebarCollapsed } from './useSidebarCollapsed'

interface NavItem {
  href: string
  label: string
  icon: LucideIcon
  /** Render a divider above this item. */
  divider?: boolean
}

export const NAV_ITEMS: NavItem[] = [
  { href: '/dashboard', label: 'Home', icon: Home },
  { href: '/dashboard/links', label: 'Links', icon: Link2 },
  { href: '/dashboard/qr-codes', label: 'QR Codes', icon: QrCode },
  { href: '/dashboard/pages', label: 'Pages', icon: FileText },
  { href: '/dashboard/analytics', label: 'Analytics', icon: BarChart2 },
  { href: '/dashboard/campaigns', label: 'Campaigns', icon: Megaphone },
  { href: '/dashboard/custom-domains', label: 'Custom domains', icon: Globe },
  { href: '/dashboard/settings', label: 'Settings', icon: Settings, divider: true },
]

export function isNavActive(href: string, pathname: string): boolean {
  if (href === '/dashboard') return pathname === '/dashboard'
  return pathname === href || pathname.startsWith(`${href}/`)
}

/** Logo, "Create new" and the nav; shared by the desktop sidebar and the mobile sheet. */
export function SidebarContent({
  collapsed = false,
  onNavigate,
}: {
  collapsed?: boolean
  onNavigate?: () => void
}) {
  const pathname = usePathname() ?? ''

  return (
    <div className="flex h-full flex-col">
      <Link
        href="/dashboard"
        onClick={onNavigate}
        aria-label={collapsed ? 'Shortener home' : undefined}
        className={cn('flex h-12 items-center gap-2 px-2 text-xl font-bold text-foreground', collapsed && 'justify-center px-0')}
      >
        <Link2 className="size-7 shrink-0 text-brand" aria-hidden />
        {!collapsed && <span>Shortener</span>}
      </Link>

      <div className="mt-4">
        <CreateNewMenu collapsed={collapsed} />
      </div>

      <nav aria-label="Main" className="mt-6 flex flex-col gap-2">
        {NAV_ITEMS.map(({ href, label, icon: Icon, divider }) => {
          const active = isNavActive(href, pathname)
          const link = (
            <Link
              href={href}
              onClick={onNavigate}
              aria-current={active ? 'page' : undefined}
              aria-label={collapsed ? label : undefined}
              className={cn(
                'relative flex h-10 w-full items-center gap-3 rounded-[4px] px-3 text-base font-medium text-sidebar-foreground transition-colors hover:bg-sidebar-active-bg/60',
                collapsed && 'justify-center px-0',
                active &&
                  'bg-sidebar-active-bg text-sidebar-active-fg before:absolute before:inset-y-0 before:left-0 before:w-[3px] before:rounded-l-[4px] before:bg-[#2A5BD7]'
              )}
            >
              <Icon className="size-5 shrink-0" aria-hidden />
              {!collapsed && <span className="truncate">{label}</span>}
            </Link>
          )
          return (
            <Fragment key={href}>
              {divider && <hr className="my-1 border-border" />}
              {collapsed ? (
                <Tooltip>
                  <TooltipTrigger asChild>{link}</TooltipTrigger>
                  <TooltipContent side="right">{label}</TooltipContent>
                </Tooltip>
              ) : (
                link
              )}
            </Fragment>
          )
        })}
      </nav>
    </div>
  )
}

/** Desktop sidebar (lg+): 240px, or 64px collapsed (state in localStorage). */
export function Sidebar() {
  const [collapsed, setCollapsed] = useSidebarCollapsed()
  useCreateShortcuts()

  return (
    <aside
      className={cn(
        'relative hidden h-screen shrink-0 bg-sidebar px-3 py-4 transition-[width] duration-200 lg:sticky lg:top-0 lg:block',
        collapsed ? 'w-16' : 'w-60'
      )}
    >
      <SidebarContent collapsed={collapsed} />
      <button
        type="button"
        onClick={() => setCollapsed(!collapsed)}
        aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        aria-expanded={!collapsed}
        className="absolute top-[88px] -right-3 z-10 flex size-6 items-center justify-center rounded-full border border-border bg-white text-foreground shadow-sm hover:bg-sidebar-active-bg"
      >
        {collapsed ? <ChevronRight className="size-4" aria-hidden /> : <ChevronLeft className="size-4" aria-hidden />}
      </button>
    </aside>
  )
}
