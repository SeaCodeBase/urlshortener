import Link from 'next/link'

export default function PublicPageNotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-background px-4 text-center">
      <h1 className="text-2xl font-semibold">This page doesn&apos;t exist</h1>
      <Link href="/" className="text-sm text-primary hover:underline">
        Go to URL Shortener
      </Link>
    </div>
  )
}
