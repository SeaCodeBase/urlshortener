'use client'

import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { Copy, LifeBuoy, LogOut, Code2 } from 'lucide-react'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export const SUPPORT_URL = 'https://github.com/SeaCodeBase/urlshortener/issues'

export function UserMenu() {
  const router = useRouter()
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)

  const emailLocal = user?.email?.split('@')[0] ?? ''
  const displayName = user?.display_name || emailLocal || 'User'
  const accountName = user?.account_name ?? emailLocal
  const initial = displayName.charAt(0).toUpperCase()

  const copyAccountName = async () => {
    try {
      await navigator.clipboard.writeText(accountName)
      toast.success('Copied to clipboard')
    } catch {
      toast.error("Couldn't copy")
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="Open user menu"
        className="flex size-8 items-center justify-center rounded-full bg-foreground text-sm font-semibold text-background outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
      >
        {initial}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72">
        <DropdownMenuLabel className="font-normal">
          <p className="truncate text-sm font-semibold text-foreground">{displayName}</p>
          <p className="truncate text-sm text-muted-foreground">{user?.email}</p>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <div className="flex items-center justify-between gap-2 px-2 py-1.5 text-sm">
          <div className="min-w-0">
            <p className="text-xs text-muted-foreground">Account</p>
            <p className="truncate text-foreground">{accountName}</p>
          </div>
          <button
            type="button"
            onClick={copyAccountName}
            aria-label="Copy account name"
            className="flex size-7 shrink-0 items-center justify-center rounded text-foreground hover:bg-accent"
          >
            <Copy className="size-4" aria-hidden />
          </button>
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <a href={SUPPORT_URL} target="_blank" rel="noopener noreferrer">
            <LifeBuoy className="size-4" aria-hidden />
            Support
          </a>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/dashboard/settings/api">
            <Code2 className="size-4" aria-hidden />
            API documentation
          </Link>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onSelect={() => {
            logout()
            router.push('/login')
          }}
        >
          <LogOut className="size-4" aria-hidden />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
