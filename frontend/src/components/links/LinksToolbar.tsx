'use client';

import { Search, List, LayoutGrid, Rows3 } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';

export type ViewMode = 'list' | 'compact' | 'grid';
export type StatusFilter = 'all' | 'active' | 'inactive';

interface LinksToolbarProps {
  search: string;
  onSearchChange: (value: string) => void;
  status: StatusFilter;
  onStatusChange: (value: StatusFilter) => void;
  viewMode: ViewMode;
  onViewModeChange: (mode: ViewMode) => void;
  selectedCount: number;
  onBulkDelete: () => void;
}

export function LinksToolbar({
  search,
  onSearchChange,
  status,
  onStatusChange,
  viewMode,
  onViewModeChange,
  selectedCount,
  onBulkDelete,
}: LinksToolbarProps) {
  return (
    <div className="space-y-3">
      {/* Search and Filters */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
          <Input
            placeholder="Search links"
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className="pl-10"
          />
        </div>
        <select
          value={status}
          onChange={(e) => onStatusChange(e.target.value as StatusFilter)}
          className="px-3 py-2 border rounded-md text-sm bg-white"
        >
          <option value="all">All links</option>
          <option value="active">Active</option>
          <option value="inactive">Inactive</option>
        </select>
      </div>

      {/* Bulk Actions and View Toggle */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="text-sm text-gray-500">
            {selectedCount} selected
          </span>
          {selectedCount > 0 && (
            <Button variant="ghost" size="sm" onClick={onBulkDelete} className="text-red-600">
              Delete
            </Button>
          )}
        </div>
        <div className="flex items-center gap-2">
          <div className="flex border rounded-md">
            <Button
              variant={viewMode === 'list' ? 'secondary' : 'ghost'}
              size="sm"
              className="rounded-r-none"
              onClick={() => onViewModeChange('list')}
            >
              <List className="w-4 h-4" />
            </Button>
            <Button
              variant={viewMode === 'compact' ? 'secondary' : 'ghost'}
              size="sm"
              className="rounded-none border-x"
              onClick={() => onViewModeChange('compact')}
            >
              <Rows3 className="w-4 h-4" />
            </Button>
            <Button
              variant={viewMode === 'grid' ? 'secondary' : 'ghost'}
              size="sm"
              className="rounded-l-none"
              onClick={() => onViewModeChange('grid')}
            >
              <LayoutGrid className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
