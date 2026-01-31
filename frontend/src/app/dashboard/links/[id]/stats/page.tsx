'use client';

import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { ArrowLeft, Copy, Check } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { api } from '@/lib/api';
import { DonutChart } from '@/components/charts/DonutChart';
import { EngagementsChart } from '@/components/charts/EngagementsChart';
import { LocationsChart } from '@/components/charts/LocationsChart';
import type { LinkStats, Link as LinkType } from '@/types';

export default function LinkStatsPage() {
  const params = useParams();
  const linkId = Number(params.id);
  const [stats, setStats] = useState<LinkStats | null>(null);
  const [link, setLink] = useState<LinkType | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

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

  const copyToClipboard = async () => {
    if (!link) return;
    await navigator.clipboard.writeText(link.short_url);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (loading) return <div className="p-8">Loading statistics...</div>;
  if (error) return <div className="p-8 text-red-500">{error}</div>;
  if (!stats || !link) return <div className="p-8">No data available</div>;

  const deviceData = stats.device_stats.map(d => ({
    name: d.device_type.charAt(0).toUpperCase() + d.device_type.slice(1),
    value: d.count,
    percentage: stats.total_clicks > 0 ? (d.count / stats.total_clicks) * 100 : 0,
  }));

  const browserData = stats.browser_stats.map(b => ({
    name: b.browser,
    value: b.count,
    percentage: stats.total_clicks > 0 ? (b.count / stats.total_clicks) * 100 : 0,
  }));

  const referrerData = stats.top_referrers.map(r => ({
    name: r.referrer,
    value: r.count,
    percentage: stats.total_clicks > 0 ? (r.count / stats.total_clicks) * 100 : 0,
  }));

  return (
    <div className="space-y-6">
      {/* Back link */}
      <Link href="/dashboard/links" className="inline-flex items-center text-sm text-gray-500 hover:text-gray-700">
        <ArrowLeft className="w-4 h-4 mr-1" />
        Back to list
      </Link>

      {/* Link Info Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex justify-between items-start">
            <div className="space-y-2">
              <h1 className="text-2xl font-bold">{link.title || 'Untitled Link'}</h1>
              <div className="flex items-center gap-2">
                <a href={link.short_url} target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline">
                  {link.short_url.replace('http://', '').replace('https://', '')}
                </a>
                <Button variant="ghost" size="sm" onClick={copyToClipboard}>
                  {copied ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                </Button>
              </div>
              <p className="text-sm text-gray-500 truncate max-w-lg">{link.original_url}</p>
              <p className="text-sm text-gray-400">
                {new Date(link.created_at).toLocaleDateString('en-US', {
                  year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit'
                })}
              </p>
            </div>
            <div className="text-right">
              <div className="text-3xl font-bold">{stats.total_clicks}</div>
              <div className="text-sm text-gray-500">Total Clicks</div>
              <div className="text-lg font-medium text-gray-600 mt-2">{stats.unique_visitors}</div>
              <div className="text-sm text-gray-500">Unique Visitors</div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Engagements Over Time */}
      <Card>
        <CardHeader>
          <CardTitle>Engagements over time</CardTitle>
        </CardHeader>
        <CardContent>
          <EngagementsChart data={stats.daily_stats} />
        </CardContent>
      </Card>

      {/* Locations */}
      <Card>
        <CardHeader>
          <CardTitle>Locations</CardTitle>
        </CardHeader>
        <CardContent>
          <LocationsChart
            countries={stats.locations?.countries || []}
            cities={stats.locations?.cities || []}
          />
        </CardContent>
      </Card>

      {/* Referrers and Devices */}
      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader>
            <CardTitle>Referrers</CardTitle>
          </CardHeader>
          <CardContent>
            <DonutChart data={referrerData} colors={['#f97316', '#14b8a6', '#3b82f6', '#8b5cf6', '#ec4899']} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Devices</CardTitle>
          </CardHeader>
          <CardContent>
            <DonutChart data={deviceData} colors={['#14b8a6', '#f97316', '#3b82f6']} />
          </CardContent>
        </Card>
      </div>

      {/* Browsers */}
      <Card>
        <CardHeader>
          <CardTitle>Browsers</CardTitle>
        </CardHeader>
        <CardContent>
          <DonutChart data={browserData} colors={['#3b82f6', '#14b8a6', '#f97316', '#8b5cf6', '#ec4899', '#10b981']} />
        </CardContent>
      </Card>
    </div>
  );
}
