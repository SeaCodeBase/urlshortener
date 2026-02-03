'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from 'lucide-react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

export default function CreateDomainPage() {
  const router = useRouter();
  const [domain, setDomain] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      await api.createDomain(domain);
      router.push('/dashboard/domains');
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : '';
      if (errorMessage.includes('409') || errorMessage.includes('already')) {
        setError('This domain is already in use');
      } else {
        setError('Failed to add domain');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 max-w-lg">
      <button
        onClick={() => router.back()}
        className="flex items-center text-gray-600 hover:text-gray-900 mb-6"
      >
        <ArrowLeft className="w-4 h-4 mr-2" />
        Back
      </button>

      <h1 className="text-2xl font-bold mb-6">Add Custom Domain</h1>

      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="bg-red-50 text-red-600 p-4 rounded">{error}</div>
        )}

        <div>
          <Label htmlFor="domain">Domain</Label>
          <Input
            id="domain"
            type="text"
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
            placeholder="short.example.com"
            required
          />
          <p className="text-sm text-gray-500 mt-1">
            Enter the domain you want to use for short URLs
          </p>
        </div>

        <Button type="submit" disabled={loading}>
          {loading ? 'Adding...' : 'Add Domain'}
        </Button>
      </form>

      <div className="mt-8 p-4 bg-gray-50 rounded">
        <h3 className="font-medium mb-2">DNS Configuration</h3>
        <p className="text-sm text-gray-600">
          After adding your domain, create a CNAME record pointing to your
          URL shortener server to enable redirects.
        </p>
      </div>
    </div>
  );
}
