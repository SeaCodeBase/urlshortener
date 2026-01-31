'use client';

import { useState, useEffect } from 'react';
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

  useEffect(() => {
    loadLinks();
  }, []);

  const loadLinks = async () => {
    try {
      const data = await api.getLinks();
      setLinks(data.links || []);
    } catch (err) {
      console.error('Failed to load links:', err);
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
      console.error('Failed to delete link:', err);
    }
  };

  const copyToClipboard = (url: string) => {
    navigator.clipboard.writeText(url);
  };

  if (loading) {
    return <div>Loading links...</div>;
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
              <Button
                variant="destructive"
                size="sm"
                onClick={() => handleDelete(link.id)}
              >
                Delete
              </Button>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
