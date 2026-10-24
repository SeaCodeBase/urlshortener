import { useCallback, useSyncExternalStore } from 'react'

const KEY = 'sidebar-collapsed'
const listeners = new Set<() => void>()

function read(): boolean {
  try {
    return window.localStorage.getItem(KEY) === 'true'
  } catch {
    return false
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

/** Sidebar collapsed state, persisted in localStorage['sidebar-collapsed']. */
export function useSidebarCollapsed(): [boolean, (collapsed: boolean) => void] {
  const collapsed = useSyncExternalStore(subscribe, read, () => false)
  const setCollapsed = useCallback((value: boolean) => {
    try {
      window.localStorage.setItem(KEY, String(value))
    } catch {
      // Storage unavailable (private mode): the toggle just won't persist.
    }
    listeners.forEach((l) => l())
  }, [])
  return [collapsed, setCollapsed]
}
