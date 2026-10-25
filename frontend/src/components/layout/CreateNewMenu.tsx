'use client'

import Link from 'next/link'
import { FileText, Link2, Plus, QrCode } from 'lucide-react'
import { cn } from '@/lib/utils'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export const CREATE_ITEMS = [
  { label: 'Link', href: '/dashboard/links/create', icon: Link2, shortcut: 'L' },
  { label: 'QR Code', href: '/dashboard/qr-codes/create', icon: QrCode, shortcut: 'Q' },
  { label: 'Page', href: '/dashboard/pages/new', icon: FileText, shortcut: undefined },
] as const

export function CreateNewMenu({ collapsed = false }: { collapsed?: boolean }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={collapsed ? 'Create new' : undefined}
        className={cn(
          'flex h-10 w-full items-center justify-center gap-2 rounded-[4px] bg-sidebar-primary text-base font-medium tracking-[0.16px] text-sidebar-primary-foreground shadow-[0_2px_8px_rgba(0,0,0,0.24)] outline-none transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-sidebar-ring focus-visible:ring-offset-2'
        )}
      >
        <Plus className="size-5 shrink-0" aria-hidden />
        {!collapsed && <span>Create new</span>}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-52">
        {CREATE_ITEMS.map(({ label, href, icon: Icon, shortcut }) => (
          <DropdownMenuItem key={href} asChild>
            <Link href={href}>
              <Icon className="size-4" aria-hidden />
              {label}
              {shortcut && <DropdownMenuShortcut>{shortcut}</DropdownMenuShortcut>}
            </Link>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
