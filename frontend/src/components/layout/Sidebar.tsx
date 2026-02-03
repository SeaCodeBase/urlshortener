'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cn } from '@/lib/utils'
import { Home, Link2, BarChart3, Settings, Plus, Globe } from 'lucide-react'
import { Button } from '@/components/ui/button'

const navItems = [
  { href: '/dashboard', label: 'Home', icon: Home },
  { href: '/dashboard/links', label: 'Links', icon: Link2 },
  { href: '/dashboard/domains', label: 'Domains', icon: Globe },
  { href: '/dashboard/analytics', label: 'Analytics', icon: BarChart3 },
  { href: '/dashboard/settings', label: 'Settings', icon: Settings },
]

export function Sidebar() {
  const pathname = usePathname()

  const isActive = (href: string) => {
    if (href === '/dashboard') {
      return pathname === '/dashboard'
    }
    return pathname.startsWith(href)
  }

  return (
    <aside className="w-[200px] border-r border-gray-200 bg-white flex flex-col h-full">
      <div className="p-4">
        <Link href="/dashboard" className="flex items-center gap-2 font-bold text-xl">
          <Link2 className="h-6 w-6 text-primary" />
          <span>Shortener</span>
        </Link>
      </div>

      <div className="px-4 mb-4">
        <Button asChild className="w-full">
          <Link href="/dashboard/links/create">
            <Plus className="h-4 w-4 mr-2" />
            Create new
          </Link>
        </Button>
      </div>

      <nav className="flex-1">
        {navItems.map((item) => {
          const Icon = item.icon
          const active = isActive(item.href)
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                'flex items-center gap-3 px-4 py-2.5 text-gray-700 hover:bg-gray-100 transition-colors',
                active && 'bg-gray-100 border-l-[3px] border-l-primary'
              )}
            >
              <Icon className={cn('h-5 w-5', active && 'text-primary')} />
              <span className={cn(active && 'font-medium')}>{item.label}</span>
            </Link>
          )
        })}
      </nav>
    </aside>
  )
}
