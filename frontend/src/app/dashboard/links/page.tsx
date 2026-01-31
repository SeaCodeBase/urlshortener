'use client'

import Link from 'next/link'
import { LinkTable } from '@/components/links/LinkTable'
import { Button } from '@/components/ui/button'
import { Plus } from 'lucide-react'

export default function LinksPage() {
  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Your Links</h1>
        <Button asChild>
          <Link href="/dashboard/links/create">
            <Plus className="h-4 w-4 mr-2" />
            Create new
          </Link>
        </Button>
      </div>

      <div className="bg-white rounded-lg border border-gray-200 p-4">
        <LinkTable />
      </div>
    </div>
  )
}
