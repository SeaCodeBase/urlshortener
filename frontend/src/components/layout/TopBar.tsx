'use client'

import { HelpCircle, Menu } from 'lucide-react'
import { GlobalSearch } from './GlobalSearch'
import { GroupSwitcher } from './GroupSwitcher'
import { UserMenu } from './UserMenu'

export const HELP_URL = 'https://github.com/SeaCodeBase/urlshortener#readme'

/** 56px bar (bg-card): hamburger (below lg) + group switcher | search, help, user menu. */
export function TopBar({ onOpenNav }: { onOpenNav?: () => void }) {
  return (
    <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center justify-between gap-2 border-b border-border bg-card py-2 pr-4 pl-6">
      <div className="flex min-w-0 items-center gap-2">
        <button
          type="button"
          onClick={onOpenNav}
          aria-label="Open navigation"
          className="-ml-2 flex size-9 items-center justify-center rounded-[4px] text-foreground hover:bg-accent lg:hidden"
        >
          <Menu className="size-5" aria-hidden />
        </button>
        <GroupSwitcher />
      </div>
      <div className="flex items-center gap-2">
        <GlobalSearch />
        <a
          href={HELP_URL}
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Help"
          className="flex size-9 items-center justify-center rounded-[4px] text-foreground hover:bg-accent"
        >
          <HelpCircle className="size-5" aria-hidden />
        </a>
        <UserMenu />
      </div>
    </header>
  )
}
