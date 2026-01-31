'use client';

import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';

interface EngagementsChartProps {
  data: Array<{
    date: string;
    clicks: number;
  }>;
}

export function EngagementsChart({ data }: EngagementsChartProps) {
  if (data.length === 0) {
    return (
      <div className="flex items-center justify-center h-[200px] text-gray-400">
        No engagement data yet
      </div>
    );
  }

  // Format date for display
  const formattedData = data.map(item => ({
    ...item,
    displayDate: new Date(item.date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
  })).reverse(); // Show oldest to newest

  return (
    <ResponsiveContainer width="100%" height={200}>
      <BarChart data={formattedData}>
        <XAxis
          dataKey="displayDate"
          tick={{ fontSize: 12 }}
          interval="preserveStartEnd"
        />
        <YAxis
          tick={{ fontSize: 12 }}
          allowDecimals={false}
        />
        <Tooltip
          labelFormatter={(label) => `Date: ${label}`}
          formatter={(value) => [value, 'Clicks']}
        />
        <Bar dataKey="clicks" fill="#14b8a6" radius={[4, 4, 0, 0]} />
      </BarChart>
    </ResponsiveContainer>
  );
}
