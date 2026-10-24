import { useCallback, useSyncExternalStore } from 'react'
import type { LinksView } from './linksUrlState'

const KEY = 'links-view'
const VIEWS: LinksView[] = ['compact', 'default', 'grid']
const listeners = new Set<() => void>()

function read(): LinksView {
  try {
    const v = window.localStorage.getItem(KEY)
    return VIEWS.includes(v as LinksView) ? (v as LinksView) : 'default'
  } catch {
    return 'default'
  }
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  window.addEventListener('storage', listener)
  return () => {
    listeners.delete(listener)
    window.removeEventListener('storage', listener)
  }
}

/** Links list view (Compact / Default / Grid), persisted in localStorage['links-view']. */
export function useLinksView(): [LinksView, (view: LinksView) => void] {
  const view = useSyncExternalStore(subscribe, read, () => 'default' as const)
  const setView = useCallback((value: LinksView) => {
    try {
      window.localStorage.setItem(KEY, value)
    } catch {
      // Storage unavailable: the choice just won't persist.
    }
    listeners.forEach((l) => l())
  }, [])
  return [view, setView]
}
