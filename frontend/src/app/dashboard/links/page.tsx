'use client';

import { useState, useEffect, useMemo } from 'react';
import Link from 'next/link';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { api } from '@/lib/api';
import { LinkCard } from '@/components/links/LinkCard';
import { LinksToolbar, ViewMode, StatusFilter } from '@/components/links/LinksToolbar';
import type { Link as LinkType } from '@/types';

export default function LinksPage() {
  const [links, setLinks] = useState<LinkType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState<StatusFilter>('all');
  const [viewMode, setViewMode] = useState<ViewMode>('list');
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());

  useEffect(() => {
    loadLinks();
  }, []);

  const loadLinks = async () => {
    try {
      setError(null);
      const data = await api.getLinks();
      setLinks(data.links || []);
    } catch (err) {
      setError('Failed to load links. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const filteredLinks = useMemo(() => {
    return links.filter(link => {
      // Status filter
      if (status === 'active' && !link.is_active) return false;
      if (status === 'inactive' && link.is_active) return false;

      // Search filter
      if (search) {
        const searchLower = search.toLowerCase();
        return (
          link.short_code.toLowerCase().includes(searchLower) ||
          link.original_url.toLowerCase().includes(searchLower) ||
          (link.title?.toLowerCase().includes(searchLower) ?? false)
        );
      }

      return true;
    });
  }, [links, search, status]);

  const handleSelect = (id: number, selected: boolean) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (selected) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to delete this link?')) return;
    try {
      await api.deleteLink(id);
      setLinks(links.filter(l => l.id !== id));
      setSelectedIds(prev => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    } catch {
      alert('Failed to delete link');
    }
  };

  const handleBulkDelete = async () => {
    if (selectedIds.size === 0) return;
    if (!confirm(`Are you sure you want to delete ${selectedIds.size} links?`)) return;

    try {
      await Promise.all(Array.from(selectedIds).map(id => api.deleteLink(id)));
      setLinks(links.filter(l => !selectedIds.has(l.id)));
      setSelectedIds(new Set());
    } catch {
      alert('Failed to delete some links');
      loadLinks();
    }
  };

  if (loading) {
    return <div className="p-8">Loading links...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Your Links</h1>
        <Button asChild>
          <Link href="/dashboard/links/create">
            <Plus className="w-4 h-4 mr-2" />
            Create link
          </Link>
        </Button>
      </div>

      {error ? (
        <div className="text-center py-8 text-red-500">
          {error}
          <Button variant="ghost" onClick={loadLinks}>Retry</Button>
        </div>
      ) : (
        <>
          <LinksToolbar
            search={search}
            onSearchChange={setSearch}
            status={status}
            onStatusChange={setStatus}
            viewMode={viewMode}
            onViewModeChange={setViewMode}
            selectedCount={selectedIds.size}
            onBulkDelete={handleBulkDelete}
          />

          {filteredLinks.length === 0 ? (
            <div className="text-center py-12 text-gray-500">
              {links.length === 0
                ? "No links yet. Create your first short link!"
                : "No links match your filters."}
            </div>
          ) : (
            <div className={
              viewMode === 'grid'
                ? 'grid grid-cols-2 gap-4'
                : 'space-y-3'
            }>
              {filteredLinks.map(link => (
                <LinkCard
                  key={link.id}
                  link={link}
                  selected={selectedIds.has(link.id)}
                  onSelect={handleSelect}
                  onDelete={handleDelete}
                />
              ))}
            </div>
          )}

          {filteredLinks.length > 0 && (
            <div className="text-center text-sm text-gray-400 py-4">
              You've reached the end of your links
            </div>
          )}
        </>
      )}
    </div>
  );
}
