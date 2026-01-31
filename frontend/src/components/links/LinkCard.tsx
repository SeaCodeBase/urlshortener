'use client';

import { useState } from 'react';
import Link from 'next/link';
import { Copy, Check, Pencil, BarChart3, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import type { Link as LinkType } from '@/types';

interface LinkCardProps {
  link: LinkType;
  selected: boolean;
  onSelect: (id: number, selected: boolean) => void;
  onDelete: (id: number) => void;
}

export function LinkCard({ link, selected, onSelect, onDelete }: LinkCardProps) {
  const [copied, setCopied] = useState(false);

  const copyToClipboard = async () => {
    await navigator.clipboard.writeText(link.short_url);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className={`border rounded-lg p-4 bg-white hover:shadow-sm transition-shadow ${selected ? 'ring-2 ring-orange-500' : ''}`}>
      <div className="flex items-start gap-3">
        <Checkbox
          checked={selected}
          onCheckedChange={(checked) => onSelect(link.id, checked)}
          className="mt-1"
        />

        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between">
            <h3 className="font-semibold text-lg truncate">
              {link.title || 'Untitled'}
            </h3>
            <div className="flex items-center gap-1">
              <Button variant="ghost" size="sm" asChild>
                <Link href={`/dashboard/links/${link.id}/edit`}>
                  <Pencil className="w-4 h-4" />
                </Link>
              </Button>
              <Button variant="ghost" size="sm" asChild>
                <Link href={`/dashboard/links/${link.id}/stats`}>
                  <BarChart3 className="w-4 h-4" />
                </Link>
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onDelete(link.id)}
                className="text-red-600 hover:text-red-700 hover:bg-red-50"
              >
                <Trash2 className="w-4 h-4" />
              </Button>
            </div>
          </div>

          <div className="flex items-center gap-2 mt-1">
            <a
              href={link.short_url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 hover:underline text-sm"
            >
              {link.short_url.replace('http://', '').replace('https://', '')}
            </a>
            <Button variant="ghost" size="sm" className="h-6 w-6 p-0" onClick={copyToClipboard}>
              {copied ? <Check className="w-3 h-3 text-green-500" /> : <Copy className="w-3 h-3" />}
            </Button>
          </div>

          <p className="text-sm text-gray-500 truncate mt-1">
            {link.original_url}
          </p>

          <div className="flex items-center gap-4 mt-3 text-xs text-gray-400">
            <span>{new Date(link.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}</span>
            <span className={`px-2 py-0.5 rounded-full ${link.is_active ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
              {link.is_active ? 'Active' : 'Inactive'}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
