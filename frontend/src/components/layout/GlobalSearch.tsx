'use client'

import { useRef, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Search, X } from 'lucide-react'

type Target = 'links' | 'qr-codes'

/** Top-bar search (spec §5.12): icon expands to an input; results open the filtered list. */
export function GlobalSearch() {
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const query = q.trim()

  const close = () => {
    setOpen(false)
    setQ('')
  }

  const go = (target: Target) => {
    if (!query) return
    router.push(`/dashboard/${target}?q=${encodeURIComponent(query)}`)
    close()
  }

  if (!open) {
    return (
      <button
        type="button"
        aria-label="Search"
        onClick={() => setOpen(true)}
        className="flex size-9 items-center justify-center rounded-[4px] text-foreground hover:bg-accent"
      >
        <Search className="size-5" aria-hidden />
      </button>
    )
  }

  return (
    <div className="relative">
      <Search className="pointer-events-none absolute top-1/2 left-3 size-5 -translate-y-1/2 text-muted-foreground" aria-hidden />
      <input
        ref={inputRef}
        type="search"
        autoFocus
        aria-label="Search links and QR Codes"
        placeholder="Search..."
        value={q}
        onChange={(e) => setQ(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            go('links')
          } else if (e.key === 'Escape') {
            close()
          }
        }}
        onBlur={() => {
          if (!query) close()
        }}
        className="h-10 w-[min(300px,60vw)] rounded-[4px] border border-border bg-white pr-9 pl-10 text-base text-foreground outline-none focus:ring-2 focus:ring-primary [&::-webkit-search-cancel-button]:hidden"
      />
      <button
        type="button"
        aria-label="Close search"
        // Keep focus in the input so blur doesn't fire first.
        onMouseDown={(e) => e.preventDefault()}
        onClick={close}
        className="absolute top-1/2 right-2 flex size-6 -translate-y-1/2 items-center justify-center rounded text-muted-foreground hover:text-foreground"
      >
        <X className="size-4" aria-hidden />
      </button>
      {query && (
        <div className="absolute top-full right-0 z-50 mt-2 w-[min(300px,60vw)] rounded-md border border-border bg-popover p-3 text-popover-foreground shadow-md">
          <p className="mb-2 truncate text-sm text-muted-foreground">{`Search for "${query}" in:`}</p>
          <div className="flex gap-2">
            {(
              [
                ['links', 'Links'],
                ['qr-codes', 'QR Codes'],
              ] as const
            ).map(([target, label]) => (
              <button
                key={target}
                type="button"
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => go(target)}
                className="h-8 rounded-[4px] border border-border bg-white px-3 text-sm font-medium text-foreground hover:bg-accent"
              >
                {label}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
