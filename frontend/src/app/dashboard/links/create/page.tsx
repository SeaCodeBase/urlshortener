'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { api } from '@/lib/api'
import { Domain } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { toast } from 'sonner'

export default function CreateLinkPage() {
  const router = useRouter()
  const [url, setUrl] = useState('')
  const [customCode, setCustomCode] = useState('')
  const [title, setTitle] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [loading, setLoading] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [domains, setDomains] = useState<Domain[]>([])
  const [selectedDomainId, setSelectedDomainId] = useState<number | null>(null)

  useEffect(() => {
    const fetchDomains = async () => {
      try {
        const response = await api.getDomains()
        setDomains(response.domains || [])
      } catch {
        // Ignore error, domains are optional
      }
    }
    fetchDomains()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!url) {
      toast.error('Please enter a URL')
      return
    }

    setLoading(true)
    try {
      await api.createLink({
        original_url: url,
        custom_code: customCode || undefined,
        title: title || undefined,
        expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
        domain_id: selectedDomainId || undefined,
      })
      toast.success('Link created successfully!')
      router.push('/dashboard/links')
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Failed to create link'
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Create a new link</h1>

      <form onSubmit={handleSubmit}>
        <Card className="mb-4">
          <CardHeader>
            <CardTitle className="text-lg">Link details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="url">Destination URL</Label>
              <Input
                id="url"
                type="url"
                placeholder="https://example.com/my-long-url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                required
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="domain">Short link domain</Label>
                <select
                  id="domain"
                  value={selectedDomainId || ''}
                  onChange={(e) => setSelectedDomainId(e.target.value ? Number(e.target.value) : null)}
                  className="w-full h-10 px-3 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                >
                  <option value="">Default</option>
                  {domains.map((d) => (
                    <option key={d.id} value={d.id}>{d.domain}</option>
                  ))}
                </select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="customCode">Back-half (optional)</Label>
                <Input
                  id="customCode"
                  placeholder="my-custom-code"
                  value={customCode}
                  onChange={(e) => setCustomCode(e.target.value)}
                  pattern="^[a-zA-Z0-9]{3,16}$"
                  title="3-16 alphanumeric characters"
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="title">Title (optional)</Label>
              <Input
                id="title"
                placeholder="My Link Title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
              />
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
                <Input
                  id="expiresAt"
                  type="datetime-local"
                  value={expiresAt}
                  onChange={(e) => setExpiresAt(e.target.value)}
                />
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
          <Button type="submit" disabled={loading}>
            {loading ? 'Creating...' : 'Create your link'}
          </Button>
        </div>
      </form>
    </div>
  )
}
