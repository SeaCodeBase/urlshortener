'use client'

import { useState, useEffect } from 'react'
import { useRouter, useParams } from 'next/navigation'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { ChevronDown, ChevronUp, Copy, Check, Info } from 'lucide-react'
import { toast } from 'sonner'
import type { Link } from '@/types'

export default function EditLinkPage() {
  const router = useRouter()
  const params = useParams()
  const linkId = Number(params.id)

  const [link, setLink] = useState<Link | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Form fields
  const [originalUrl, setOriginalUrl] = useState('')
  const [title, setTitle] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [isActive, setIsActive] = useState(true)

  // Short code editing
  const [shortCode, setShortCode] = useState('')
  const [isEditingShortCode, setIsEditingShortCode] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    loadLink()
  }, [linkId])

  const loadLink = async () => {
    try {
      const data = await api.getLink(linkId)
      setLink(data)
      setOriginalUrl(data.original_url)
      setTitle(data.title || '')
      setShortCode(data.short_code)
      setIsActive(data.is_active)
      if (data.expires_at) {
        // Convert to datetime-local format
        const date = new Date(data.expires_at)
        setExpiresAt(date.toISOString().slice(0, 16))
      }
    } catch (err) {
      setError('Failed to load link')
    } finally {
      setLoading(false)
    }
  }

  const handleCopy = async () => {
    if (link) {
      await navigator.clipboard.writeText(link.short_url)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!link) return

    setSaving(true)
    try {
      // Check if short code changed
      if (shortCode !== link.short_code) {
        // Create a new link with the new short code (clone behavior)
        await api.createLink({
          original_url: originalUrl,
          custom_code: shortCode,
          title: title || undefined,
          expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
        })
        toast.success('New short link created! The original link remains active.')
      } else {
        // Just update the existing link
        await api.updateLink(linkId, {
          original_url: originalUrl,
          title: title,
          expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
          is_active: isActive,
        })
        toast.success('Link updated successfully!')
      }
      router.push('/dashboard/links')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to save link'
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-gray-500">Loading...</div>
      </div>
    )
  }

  if (error || !link) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-red-500">{error || 'Link not found'}</div>
      </div>
    )
  }

  const shortCodeChanged = shortCode !== link.short_code

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Edit link</h1>

      <form onSubmit={handleSubmit}>
        {/* Info banner when editing short code */}
        {shortCodeChanged && (
          <div className="mb-4 p-4 bg-blue-50 border border-blue-200 rounded-lg flex items-start gap-3">
            <Info className="h-5 w-5 text-blue-600 mt-0.5 flex-shrink-0" />
            <p className="text-sm text-blue-800">
              Editing your short link will create a new, separate short link. The current short link will remain active and continue to point to the same destination.
            </p>
          </div>
        )}

        <Card className="mb-4">
          <CardHeader>
            <CardTitle className="text-lg">Short link</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="domain">Domain</Label>
                <Input
                  id="domain"
                  value="localhost:8080"
                  disabled
                  className="bg-gray-50"
                />
                <p className="text-xs text-gray-500">
                  The domain cannot be changed after a link has been shortened.
                </p>
              </div>
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label htmlFor="shortCode">Back-half</Label>
                  {!isEditingShortCode && (
                    <button
                      type="button"
                      className="text-sm text-blue-600 hover:text-blue-700"
                      onClick={() => setIsEditingShortCode(true)}
                    >
                      Edit back-half
                    </button>
                  )}
                </div>
                {isEditingShortCode ? (
                  <Input
                    id="shortCode"
                    value={shortCode}
                    onChange={(e) => setShortCode(e.target.value)}
                    pattern="^[a-zA-Z0-9]{3,16}$"
                    title="3-16 alphanumeric characters"
                  />
                ) : (
                  <div className="flex items-center gap-2">
                    <Input
                      id="shortCode"
                      value={shortCode}
                      disabled
                      className="bg-gray-50"
                    />
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      onClick={handleCopy}
                    >
                      {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    </Button>
                  </div>
                )}
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="mb-4">
          <CardHeader>
            <CardTitle className="text-lg">Destination URL</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <Input
                id="originalUrl"
                type="url"
                placeholder="https://example.com/my-long-url"
                value={originalUrl}
                onChange={(e) => setOriginalUrl(e.target.value)}
                required
              />
            </div>
          </CardContent>
        </Card>

        <Card className="mb-4">
          <CardHeader>
            <CardTitle className="text-lg">Optional details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="title">Title</Label>
              <Input
                id="title"
                placeholder="My Link Title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
              />
            </div>

            <div className="flex items-center space-x-2">
              <Checkbox
                id="isActive"
                checked={isActive}
                onCheckedChange={(checked) => setIsActive(checked === true)}
              />
              <Label htmlFor="isActive" className="font-normal">
                Link is active
              </Label>
            </div>
          </CardContent>
        </Card>

        <Card className="mb-6">
          <CardHeader
            className="cursor-pointer flex flex-row items-center justify-between"
            onClick={() => setShowAdvanced(!showAdvanced)}
          >
            <CardTitle className="text-lg">Advanced settings</CardTitle>
            {showAdvanced ? <ChevronUp className="h-5 w-5" /> : <ChevronDown className="h-5 w-5" />}
          </CardHeader>
          {showAdvanced && (
            <CardContent>
              <div className="space-y-2">
                <Label htmlFor="expiresAt">Expiration date (optional)</Label>
                <div className="flex gap-2">
                  <Input
                    id="expiresAt"
                    type="datetime-local"
                    value={expiresAt}
                    onChange={(e) => setExpiresAt(e.target.value)}
                    className="flex-1"
                  />
                  {expiresAt && (
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => setExpiresAt('')}
                    >
                      Clear
                    </Button>
                  )}
                </div>
              </div>
            </CardContent>
          )}
        </Card>

        <div className="flex justify-end gap-3">
          <Button
            type="button"
            variant="outline"
            onClick={() => router.push('/dashboard/links')}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={saving}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </div>
      </form>
    </div>
  )
}
