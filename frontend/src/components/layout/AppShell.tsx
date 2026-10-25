'use client'

import { useState, type ReactNode } from 'react'
import { useGlobalShortcuts } from '@/lib/hooks/useGlobalShortcuts'
import { Sheet, SheetContent, SheetDescription, SheetTitle } from '@/components/ui/sheet'
import { Sidebar, SidebarContent } from './Sidebar'
import { TopBar } from './TopBar'

/** Sidebar (off-canvas sheet below lg) + top bar + content on the canvas. */
export function AppShell({ children }: { children: ReactNode }) {
  const [navOpen, setNavOpen] = useState(false)
  useGlobalShortcuts()

  return (
    <div className="flex min-h-screen bg-background">
      <Sidebar />
      <Sheet open={navOpen} onOpenChange={setNavOpen}>
        <SheetContent side="left" className="w-60 bg-sidebar px-3 py-4">
          <SheetTitle className="sr-only">Navigation</SheetTitle>
          <SheetDescription className="sr-only">Main navigation</SheetDescription>
          <SidebarContent onNavigate={() => setNavOpen(false)} />
        </SheetContent>
      </Sheet>
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar onOpenNav={() => setNavOpen(true)} />
        <main className="flex-1 px-10 pb-10">{children}</main>
      </div>
    </div>
  )
}
