'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';

interface LocationsChartProps {
  countries: Array<{
    code: string;
    name: string;
    clicks: number;
    percentage: number;
  }>;
  cities: Array<{
    name: string;
    country: string;
    clicks: number;
    percentage: number;
  }>;
}

export function LocationsChart({ countries, cities }: LocationsChartProps) {
  const [view, setView] = useState<'countries' | 'cities'>('countries');

  const data = view === 'countries' ? countries : cities;
  const maxCount = Math.max(...data.map(d => d.clicks), 1);

  if (data.length === 0) {
    return (
      <div className="flex items-center justify-center h-[200px] text-gray-400">
        No location data yet
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        <Button
          variant={view === 'countries' ? 'default' : 'outline'}
          size="sm"
          onClick={() => setView('countries')}
        >
          Countries
        </Button>
        <Button
          variant={view === 'cities' ? 'default' : 'outline'}
          size="sm"
          onClick={() => setView('cities')}
        >
          Cities
        </Button>
      </div>
      <div className="space-y-3">
        {data.map((item, index) => (
          <div key={view === 'countries' ? (item as typeof countries[0]).code : `${item.name}-${(item as typeof cities[0]).country}`} className="space-y-1">
            <div className="flex justify-between text-sm">
              <span className="flex items-center gap-2">
                <span className="text-gray-500 w-4">{index + 1}.</span>
                {view === 'countries' ? (item as typeof countries[0]).name : `${item.name}, ${(item as typeof cities[0]).country}`}
              </span>
              <span className="text-gray-500">
                {item.clicks} ({item.percentage.toFixed(0)}%)
              </span>
            </div>
            <div className="w-full bg-gray-100 rounded-full h-2">
              <div
                className="bg-teal-500 h-2 rounded-full transition-all"
                style={{ width: `${(item.clicks / maxCount) * 100}%` }}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
