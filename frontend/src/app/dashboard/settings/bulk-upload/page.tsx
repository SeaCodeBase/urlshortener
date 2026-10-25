'use client'

import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { BulkJobRow } from '@/components/settings/BulkJobRow'
import { BulkUploadForm } from '@/components/settings/BulkUploadForm'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useBulkUploads } from '@/lib/hooks'

export default function BulkUploadPage() {
  const { data: uploads, isLoading, isError } = useBulkUploads()

  return (
    <>
      <PageHeader title="Bulk upload" />
      <div className="space-y-6">
        <BulkUploadForm />
        <section className="space-y-3">
          <h2 className="text-xl font-semibold text-foreground">Uploads</h2>
          {isLoading ? (
            <Skeleton className="h-48 w-full rounded-xl" aria-label="Loading uploads" />
          ) : isError ? (
            <p role="alert" className="rounded-xl bg-card p-6 text-sm text-destructive">
              Failed to load uploads
            </p>
          ) : !uploads?.length ? (
            <EmptyState title="No uploads yet" description="Your bulk upload jobs will appear here." />
          ) : (
            <div className="rounded-xl bg-card p-4">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>File</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Progress</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead>
                      <span className="sr-only">Actions</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {uploads.map((u) => (
                    <BulkJobRow key={u.id} upload={u} />
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </section>
      </div>
    </>
  )
}
