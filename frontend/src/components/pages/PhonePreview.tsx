'use client'

import type { MouseEvent } from 'react'
import { cn } from '@/lib/utils'
import type { Page } from '@/types'
import { PagePreview } from './PagePreview'

/** Editor links go nowhere: cancel any anchor click inside the preview. */
function cancelLinks(e: MouseEvent) {
  if ((e.target as HTMLElement).closest('a')) e.preventDefault()
}

/** The page's draft (every block, inactive ones dimmed) inside a 375 px phone frame. */
export function PhonePreview({ page, className }: { page: Page; className?: string }) {
  return (
    <div
      className={cn('h-[700px] w-[375px] overflow-hidden rounded-[32px] border-8 border-foreground/80 bg-background', className)}
      onClickCapture={cancelLinks}
    >
      <div className="h-full overflow-y-auto">
        <PagePreview
          mode="editor"
          page={{
            title: page.title,
            description: page.description,
            avatar_url: page.avatar_url,
            design: page.design,
            blocks: page.blocks ?? [],
          }}
        />
      </div>
    </div>
  )
}
