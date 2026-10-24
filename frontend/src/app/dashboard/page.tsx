'use client'

import { GettingStarted } from '@/components/home/GettingStarted'
import { QuickCreate } from '@/components/home/QuickCreate'
import { RecentLinks } from '@/components/home/RecentLinks'
import { TodayStats } from '@/components/home/TodayStats'
import { useAuthStore } from '@/stores/auth'

export default function HomePage() {
  const dismissed = useAuthStore((s) => s.user?.home_checklist_dismissed ?? false)
  return (
    <div className="mx-auto w-full max-w-[1040px] space-y-6 pt-8 pb-12">
      <QuickCreate />
      <div className={dismissed ? 'grid gap-6' : 'grid gap-6 md:grid-cols-2'}>
        <TodayStats />
        <GettingStarted />
      </div>
      <RecentLinks />
    </div>
  )
}
