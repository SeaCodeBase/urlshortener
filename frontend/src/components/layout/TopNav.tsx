'use client'

import { useState } from 'react'
import { useAuthStore } from '@/stores/auth'
import { HelpCircle, LogOut, Copy, Check } from 'lucide-react'
import { Button } from '@/components/ui/button'

export function TopNav() {
  const { user, logout } = useAuthStore()
  const [showMenu, setShowMenu] = useState(false)
  const [copied, setCopied] = useState(false)

  const displayName = user?.display_name || user?.email?.split('@')[0] || 'User'
  const initial = displayName.charAt(0).toUpperCase()

  const copyUserId = async () => {
    if (user?.id) {
      await navigator.clipboard.writeText(String(user.id))
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  return (
    <header className="h-14 border-b border-gray-200 bg-white flex items-center justify-between px-6">
      <div>{/* Breadcrumb placeholder */}</div>

      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" className="text-gray-500 hover:text-gray-700">
          <HelpCircle className="h-5 w-5" />
        </Button>

        <div className="relative">
          <button
            onClick={() => setShowMenu(!showMenu)}
            className="w-8 h-8 rounded-full bg-primary text-white flex items-center justify-center font-semibold text-sm hover:bg-primary/90 transition-colors"
          >
            {initial}
          </button>

          {showMenu && (
            <>
              <div
                className="fixed inset-0 z-10"
                onClick={() => setShowMenu(false)}
              />
              <div className="absolute right-0 top-10 w-64 bg-white border border-gray-200 rounded-lg shadow-lg z-20">
                <div className="p-4 border-b border-gray-100">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-full bg-primary text-white flex items-center justify-center font-semibold">
                      {initial}
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="font-medium text-gray-900 truncate">{displayName}</p>
                      <p className="text-sm text-gray-500 truncate">{user?.email}</p>
                    </div>
                  </div>
                </div>

                <div className="p-3 border-b border-gray-100">
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-gray-500">User ID: {user?.id}</span>
                    <button
                      onClick={copyUserId}
                      className="text-gray-400 hover:text-gray-600"
                    >
                      {copied ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}
                    </button>
                  </div>
                </div>

                <div className="p-2">
                  <button
                    onClick={() => {
                      setShowMenu(false)
                      logout()
                    }}
                    className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded-md transition-colors"
                  >
                    <LogOut className="h-4 w-4" />
                    Sign out
                  </button>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </header>
  )
}
