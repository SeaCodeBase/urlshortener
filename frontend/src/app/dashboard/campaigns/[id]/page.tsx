'use client'

import { useMemo, useState } from 'react'
import NextLink from 'next/link'
import { useParams } from 'next/navigation'
import { ChevronLeft } from 'lucide-react'
import { CampaignDialog } from '@/components/campaigns/CampaignDialog'
import { CampaignLinksTable } from '@/components/campaigns/CampaignLinksTable'
import {
  CampaignPerformance,
  campaignStatsQuery,
  DEFAULT_CAMPAIGN_RANGE,
} from '@/components/campaigns/CampaignPerformance'
import { ChannelCard } from '@/components/campaigns/ChannelCard'
import { EmptyState } from '@/components/common/EmptyState'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useCampaign, useCampaignStats } from '@/lib/hooks'
import type { CampaignDetail, DashboardDateRange } from '@/types'

const CAMPAIGNS_HREF = '/dashboard/campaigns'

function BackLink() {
  return (
    <NextLink
      href={CAMPAIGNS_HREF}
      className="inline-flex items-center gap-1 text-sm font-semibold text-primary hover:underline"
    >
      <ChevronLeft className="size-4" aria-hidden />
      Campaigns
    </NextLink>
  )
}

function DetailSkeleton() {
  return (
    <div className="space-y-6 pt-8" aria-busy="true" aria-label="Loading campaign">
      <Skeleton className="h-5 w-24" />
      <Skeleton className="h-9 w-72" />
      <Skeleton className="h-10 w-48" />
      <Skeleton className="h-40 rounded-xl" />
      <Skeleton className="h-40 rounded-xl" />
    </div>
  )
}

function CampaignView({ campaign }: { campaign: CampaignDetail }) {
  const [editOpen, setEditOpen] = useState(false)
  const [range, setRange] = useState<DashboardDateRange>(DEFAULT_CAMPAIGN_RANGE)
  const query = useMemo(() => campaignStatsQuery(range), [range])
  // Same query as the Performance card (deduped by react-query): feeds the channel headers.
  const { data: stats } = useCampaignStats(campaign.id, query)
  const statsByChannel = new Map(stats?.channels.map((c) => [c.channel_id, c]) ?? [])

  return (
    <div className="space-y-6 pt-8 pb-12">
      <BackLink />
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <h1 className="text-[32px] leading-10 font-bold text-foreground">{campaign.name}</h1>
          {campaign.description && <p className="mt-1 text-base text-muted-foreground">{campaign.description}</p>}
        </div>
        <Button type="button" variant="secondary" onClick={() => setEditOpen(true)}>
          Edit
        </Button>
      </div>

      <Tabs defaultValue="channels" className="min-w-0">
        <TabsList>
          <TabsTrigger value="channels">Channels</TabsTrigger>
          <TabsTrigger value="links">Links</TabsTrigger>
        </TabsList>
        <TabsContent value="channels" className="pt-4">
          {campaign.channels.length === 0 ? (
            <EmptyState
              title="No channels yet"
              description="Edit the campaign to add channels."
              action={
                <Button type="button" onClick={() => setEditOpen(true)}>
                  Edit
                </Button>
              }
            />
          ) : (
            <div className="flex flex-col gap-4">
              {campaign.channels.map((ch) => (
                <ChannelCard key={ch.id} campaign={campaign} channel={ch} stats={statsByChannel.get(ch.id)} />
              ))}
            </div>
          )}
        </TabsContent>
        <TabsContent value="links" className="pt-4">
          <CampaignLinksTable campaign={campaign} />
        </TabsContent>
      </Tabs>

      <CampaignPerformance campaignId={campaign.id} range={range} onRangeChange={setRange} />

      <CampaignDialog open={editOpen} onOpenChange={setEditOpen} campaign={campaign} />
    </div>
  )
}

/** Campaign detail: header (Edit), Channels | Links tabs, and the Performance card. */
export default function CampaignDetailPage() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)
  const valid = Number.isInteger(id) && id > 0
  const { data: campaign, isLoading, isError } = useCampaign(valid ? id : null)

  if (valid && isLoading) return <DetailSkeleton />
  if (!valid || isError || !campaign) {
    return (
      <div className="space-y-4 pt-8">
        <BackLink />
        <p role="alert" className="rounded-xl bg-card p-6 text-destructive">
          Failed to load campaign
        </p>
      </div>
    )
  }
  return <CampaignView campaign={campaign} />
}
