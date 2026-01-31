'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { LinkTable } from '@/components/links/LinkTable';
import { api } from '@/lib/api';

export default function DashboardPage() {
  const [url, setUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [refreshKey, setRefreshKey] = useState(0);

  const handleQuickCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await api.createLink({ original_url: url });
      setUrl('');
      // Trigger table refresh
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
          <CardTitle>Quick Create</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleQuickCreate} className="flex gap-4">
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
          </form>
          {error && (
            <p className="mt-2 text-sm text-red-500">{error}</p>
          )}
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
