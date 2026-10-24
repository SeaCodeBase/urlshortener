import Link from 'next/link'
import { Link2 } from 'lucide-react'

/** Bitly-style auth layout: form column on the left, brand illustration on the right (≥ lg). */
export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="grid min-h-screen bg-white lg:grid-cols-2 dark:bg-background">
      <div className="flex flex-col px-4 py-8 sm:px-8">
        <Link href="/" className="flex items-center gap-2 text-xl font-bold text-foreground">
          <Link2 className="size-7 text-brand" aria-hidden />
          <span>Shortener</span>
        </Link>
        <main className="flex flex-1 items-center justify-center py-10">
          <div className="w-full max-w-[400px]">{children}</div>
        </main>
      </div>
      <aside
        aria-hidden
        className="hidden flex-col items-center justify-center gap-8 bg-sidebar-primary px-12 text-center text-sidebar-primary-foreground lg:flex"
      >
        <Link2 className="size-40" strokeWidth={1.5} />
        <p className="max-w-sm text-base opacity-80">Connect your audience to everything you share.</p>
      </aside>
    </div>
  )
}
