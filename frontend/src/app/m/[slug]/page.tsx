import { cache } from 'react'
import type { Metadata } from 'next'
import { notFound } from 'next/navigation'
import type { PublishedPage } from '@/types'
import { isValidSlug } from '@/components/pages/designPresets'
import { PublicPageView } from '@/components/pages/PublicPageView'
import { ViewBeacon } from '@/components/pages/ViewBeacon'

export const dynamic = 'force-dynamic'

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:8081'

type Params = { params: Promise<{ slug: string }> }

/** The published page, or null for an invalid slug, unpublished page or backend error. Deduped per request. */
const loadPage = cache(async (raw: string): Promise<PublishedPage | null> => {
  // Slugs are stored lowercase; SLUG_RE + length check before any backend call.
  const slug = raw.toLowerCase()
  if (!isValidSlug(slug)) return null
  try {
    const res = await fetch(`${BACKEND_URL}/api/public/pages/${encodeURIComponent(slug)}`, { cache: 'no-store' })
    if (res.status !== 200) return null
    return (await res.json()) as PublishedPage
  } catch {
    return null
  }
})

export async function generateMetadata({ params }: Params): Promise<Metadata> {
  const { slug } = await params
  const page = await loadPage(slug)
  if (!page) return { title: 'Page not found' }
  return {
    title: page.title || slug,
    description: page.description || undefined,
    robots: { index: true },
  }
}

export default async function PublicPage({ params }: Params) {
  const { slug } = await params
  const page = await loadPage(slug)
  if (!page) notFound()
  return (
    <>
      <PublicPageView page={page} />
      <ViewBeacon slug={page.slug || slug} />
    </>
  )
}
