import Link from 'next/link'
import type { PublishedPage } from '@/types'
import { PagePreview, designVars } from './PagePreview'

/** The published link-in-bio page: full-viewport canvas in the page design, centred 480 px column. */
export function PublicPageView({ page }: { page: PublishedPage }) {
  const vars = designVars(page.design)
  return (
    <div className="flex min-h-screen flex-col" style={{ background: vars['--page-bg'], color: page.design.text_color }}>
      <main className="mx-auto w-full max-w-[480px] flex-1 px-4 py-10">
        <PagePreview page={page} mode="public" />
      </main>
      <footer className="pb-8 text-center text-xs opacity-70">
        <Link href="/" className="hover:underline">
          Made with URL Shortener
        </Link>
      </footer>
    </div>
  )
}
