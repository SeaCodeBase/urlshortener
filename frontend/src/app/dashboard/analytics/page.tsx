'use client'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { BarChart3 } from 'lucide-react'

export default function AnalyticsPage() {
  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Analytics</h1>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BarChart3 className="h-5 w-5" />
            Coming Soon
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-gray-500">
            The analytics dashboard is under development. Check back soon for detailed
            insights about your link performance, including:
          </p>
          <ul className="mt-4 space-y-2 text-gray-600">
            <li>• Total clicks across all links</li>
            <li>• Click trends over time</li>
            <li>• Geographic distribution</li>
            <li>• Device and browser breakdown</li>
            <li>• Top performing links</li>
          </ul>
        </CardContent>
      </Card>
    </div>
  )
}
