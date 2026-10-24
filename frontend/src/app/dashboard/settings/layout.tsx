import type { ReactNode } from 'react'
import { SettingsNav } from '@/components/settings/SettingsNav'

export default function SettingsLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-col gap-8 md:flex-row">
      <aside className="shrink-0 pt-8 md:w-[240px]">
        <SettingsNav />
      </aside>
      <div className="min-w-0 max-w-[720px] flex-1 pb-12">{children}</div>
    </div>
  )
}
