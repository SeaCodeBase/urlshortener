'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { LinkTable } from '@/components/links/LinkTable';
import { api } from '@/lib/api';

export default function DashboardPage() {
  const [url, setUrl] = useState('');
  const [customCode, setCustomCode] = useState('');
  const [title, setTitle] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [refreshKey, setRefreshKey] = useState(0);

  const handleQuickCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      // Convert datetime-local to RFC3339 format
      // datetime-local format: "2026-02-28T23:59"
      // RFC3339 format needed: "2026-02-28T23:59:00Z"
      const expiresAtRFC3339 = expiresAt ? expiresAt + ':00Z' : undefined;

      await api.createLink({
        original_url: url,
        custom_code: customCode || undefined,
        title: title || undefined,
        expires_at: expiresAtRFC3339,
      });
      setUrl('');
      setCustomCode('');
      setTitle('');
      setExpiresAt('');
      setRefreshKey(k => k + 1);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create link');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Create Short Link</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleQuickCreate} className="space-y-4">
            <div className="flex gap-4">
              <Input
                type="url"
                placeholder="Enter URL to shorten"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                className="flex-1"
                required
              />
              <Button type="submit" disabled={loading}>
                {loading ? 'Creating...' : 'Shorten'}
              </Button>
            </div>

            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => setShowAdvanced(!showAdvanced)}
            >
              {showAdvanced ? 'Hide' : 'Show'} Advanced Options
            </Button>

            {showAdvanced && (
              <div className="grid gap-4 pt-2 border-t">
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="customCode">Custom Short Code (optional)</Label>
                    <Input
                      id="customCode"
                      placeholder="my-custom-url"
                      value={customCode}
                      onChange={(e) => setCustomCode(e.target.value)}
                      pattern="[a-zA-Z0-9]{4,16}"
                      title="4-16 alphanumeric characters"
                    />
                    <p className="text-xs text-gray-500">4-16 alphanumeric characters</p>
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
                </div>
                <div className="space-y-2">
                  <Label htmlFor="expiresAt">Expires At (optional)</Label>
                  <Input
                    id="expiresAt"
                    type="datetime-local"
                    value={expiresAt}
                    onChange={(e) => setExpiresAt(e.target.value)}
                    min={new Date().toISOString().slice(0, 16)}
                  />
                </div>
              </div>
            )}

            {error && (
              <p className="text-sm text-red-500">{error}</p>
            )}
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Your Links</CardTitle>
        </CardHeader>
        <CardContent>
          <LinkTable key={refreshKey} />
        </CardContent>
      </Card>
    </div>
  );
}
