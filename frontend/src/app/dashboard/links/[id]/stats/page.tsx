'use client';

import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { api } from '@/lib/api';
import type { LinkStats, Link as LinkType } from '@/types';

export default function LinkStatsPage() {
  const params = useParams();
  const linkId = Number(params.id);
  const [stats, setStats] = useState<LinkStats | null>(null);
  const [link, setLink] = useState<LinkType | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadData();
  }, [linkId]);

  const loadData = async () => {
    try {
      const [statsData, linkData] = await Promise.all([
        api.getLinkStats(linkId),
        api.getLink(linkId),
      ]);
      setStats(statsData);
      setLink(linkData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load stats');
    } finally {
      setLoading(false);
    }
  };

  if (loading) return <div>Loading statistics...</div>;
  if (error) return <div className="text-red-500">{error}</div>;
  if (!stats || !link) return <div>No data available</div>;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Link Statistics</h1>
          <p className="text-gray-600">{link.short_url}</p>
          <p className="text-sm text-gray-500 truncate max-w-md">{link.original_url}</p>
        </div>
        <Link href="/dashboard">
          <Button variant="outline">Back to Dashboard</Button>
        </Link>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-gray-500">Total Clicks</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{stats.total_clicks}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-gray-500">Unique Visitors</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{stats.unique_visitors}</div>
          </CardContent>
        </Card>
      </div>

      {/* Daily Stats */}
      <Card>
        <CardHeader>
          <CardTitle>Clicks Over Time (Last 30 Days)</CardTitle>
        </CardHeader>
        <CardContent>
          {stats.daily_stats.length === 0 ? (
            <p className="text-gray-500">No click data yet</p>
          ) : (
            <div className="space-y-2">
              {stats.daily_stats.map((day) => (
                <div key={day.date} className="flex items-center gap-4">
                  <span className="w-24 text-sm text-gray-600">{day.date}</span>
                  <div className="flex-1 bg-gray-100 rounded-full h-4">
                    <div
                      className="bg-blue-500 h-4 rounded-full"
                      style={{
                        width: `${Math.min(100, (day.clicks / Math.max(...stats.daily_stats.map(d => d.clicks))) * 100)}%`,
                      }}
                    />
                  </div>
                  <span className="w-12 text-sm font-medium">{day.clicks}</span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Breakdown Cards */}
      <div className="grid grid-cols-3 gap-4">
        {/* Top Referrers */}
        <Card>
          <CardHeader>
            <CardTitle>Top Referrers</CardTitle>
          </CardHeader>
          <CardContent>
            {stats.top_referrers.length === 0 ? (
              <p className="text-gray-500 text-sm">No data</p>
            ) : (
              <ul className="space-y-2">
                {stats.top_referrers.map((ref) => (
                  <li key={ref.referrer} className="flex justify-between text-sm">
                    <span className="truncate max-w-[150px]">{ref.referrer}</span>
                    <span className="font-medium">{ref.count}</span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        {/* Devices */}
        <Card>
          <CardHeader>
            <CardTitle>Devices</CardTitle>
          </CardHeader>
          <CardContent>
            {stats.device_stats.length === 0 ? (
              <p className="text-gray-500 text-sm">No data</p>
            ) : (
              <ul className="space-y-2">
                {stats.device_stats.map((device) => (
                  <li key={device.device_type} className="flex justify-between text-sm">
                    <span className="capitalize">{device.device_type}</span>
                    <span className="font-medium">{device.count}</span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        {/* Browsers */}
        <Card>
          <CardHeader>
            <CardTitle>Browsers</CardTitle>
          </CardHeader>
          <CardContent>
            {stats.browser_stats.length === 0 ? (
              <p className="text-gray-500 text-sm">No data</p>
            ) : (
              <ul className="space-y-2">
                {stats.browser_stats.map((browser) => (
                  <li key={browser.browser} className="flex justify-between text-sm">
                    <span>{browser.browser}</span>
                    <span className="font-medium">{browser.count}</span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
