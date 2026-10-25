'use client'

import { useState } from 'react'
import { CampaignCard } from '@/components/campaigns/CampaignCard'
import { CampaignDialog } from '@/components/campaigns/CampaignDialog'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useCampaigns } from '@/lib/hooks'

function CardSkeleton() {
  return (
    <div className="flex flex-col gap-2 rounded-xl bg-card p-6" aria-hidden>
      <Skeleton className="h-6 w-1/3" />
      <Skeleton className="h-4 w-1/2" />
      <Skeleton className="mt-2 h-4 w-1/4" />
    </div>
  )
}

export default function CampaignsPage() {
  const query = useCampaigns()
  const [createOpen, setCreateOpen] = useState(false)
  const campaigns = query.data ?? []

  let body
  if (query.isPending) {
    body = (
      <div className="flex flex-col gap-4" aria-busy="true" aria-label="Loading campaigns">
        <CardSkeleton />
        <CardSkeleton />
        <CardSkeleton />
      </div>
    )
  } else if (query.isError) {
    body = (
      <p role="alert" className="rounded-xl bg-card p-6 text-destructive">
        Failed to load campaigns
      </p>
    )
  } else if (campaigns.length === 0) {
    body = (
      <EmptyState
        title="Organize links into campaigns"
        description="Group links by channel, add UTM parameters automatically and compare how each channel performs."
        action={
          <Button type="button" onClick={() => setCreateOpen(true)}>
            Create your first campaign
          </Button>
        }
      />
    )
  } else {
    body = (
      <div className="flex flex-col gap-4">
        {campaigns.map((c) => (
          <CampaignCard key={c.id} campaign={c} />
        ))}
      </div>
    )
  }

  return (
    <div className="pb-12">
      <PageHeader
        title="Campaigns"
        actions={
          <Button type="button" onClick={() => setCreateOpen(true)}>
            Create campaign
          </Button>
        }
      />
      {body}
      <CampaignDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  )
}
