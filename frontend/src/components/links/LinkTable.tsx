'use client';

import { useState, useEffect } from 'react';
import NextLink from 'next/link';
import { api } from '@/lib/api';
import type { Link } from '@/types';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Button } from '@/components/ui/button';

export function LinkTable() {
  const [links, setLinks] = useState<Link[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadLinks();
  }, []);

  const loadLinks = async () => {
    try {
      setError(null);
      const data = await api.getLinks();
      setLinks(data.links || []);
    } catch (err) {
      console.error('Failed to load links:', err);
      setError('Failed to load links. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to delete this link?')) return;

    try {
      await api.deleteLink(id);
      setLinks(links.filter((l) => l.id !== id));
    } catch (err) {
      alert('Failed to delete link. Please try again.');
      console.error('Failed to delete link:', err);
    }
  };

  const copyToClipboard = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url);
      // Simple feedback - could use toast in future
    } catch {
      alert('Failed to copy to clipboard');
    }
  };

  if (loading) {
    return <div>Loading links...</div>;
  }

  if (error) {
    return (
      <div className="text-center py-8 text-red-500">
        {error}
        <Button variant="ghost" onClick={() => { setError(null); loadLinks(); }}>
          Retry
        </Button>
      </div>
    );
  }

  if (links.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        No links yet. Create your first short link!
      </div>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Short URL</TableHead>
          <TableHead>Original URL</TableHead>
          <TableHead>Created</TableHead>
          <TableHead>Expires</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {links.map((link) => (
          <TableRow key={link.id}>
            <TableCell>
              <div className="flex items-center gap-2">
                <a
                  href={link.short_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-600 hover:underline"
                >
                  {link.short_code}
                </a>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => copyToClipboard(link.short_url)}
                >
                  Copy
                </Button>
              </div>
            </TableCell>
            <TableCell className="max-w-xs truncate">
              {link.original_url}
            </TableCell>
            <TableCell>
              {new Date(link.created_at).toLocaleDateString()}
            </TableCell>
            <TableCell>
              {link.expires_at ? (
                <span className={new Date(link.expires_at) < new Date() ? 'text-red-500' : ''}>
                  {new Date(link.expires_at).toLocaleDateString()}
                </span>
              ) : (
                <span className="text-gray-400">Never</span>
              )}
            </TableCell>
            <TableCell>
              <span
                className={`px-2 py-1 rounded-full text-xs ${
                  link.is_active
                    ? 'bg-green-100 text-green-800'
                    : 'bg-red-100 text-red-800'
                }`}
              >
                {link.is_active ? 'Active' : 'Inactive'}
              </span>
            </TableCell>
            <TableCell>
              <div className="flex gap-2">
                <NextLink href={`/dashboard/links/${link.id}/stats`}>
                  <Button variant="outline" size="sm">
                    Stats
                  </Button>
                </NextLink>
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={() => handleDelete(link.id)}
                >
                  Delete
                </Button>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
