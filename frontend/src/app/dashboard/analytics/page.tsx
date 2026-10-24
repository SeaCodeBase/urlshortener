import { OverviewDashboards } from '@/components/analytics/OverviewDashboards'
import { PerformanceOverview } from '@/components/analytics/PerformanceOverview'
import { TopPerforming } from '@/components/analytics/TopPerforming'
import { PageHeader } from '@/components/common/PageHeader'

export default function AnalyticsPage() {
  return (
    <div className="pb-12">
      <PageHeader title="Analytics" />
      <div className="flex flex-col gap-10">
        <OverviewDashboards />
        <PerformanceOverview />
        <TopPerforming />
      </div>
    </div>
  )
}
