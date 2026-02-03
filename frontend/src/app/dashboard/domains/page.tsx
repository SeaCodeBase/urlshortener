'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Plus, Trash2, Globe } from 'lucide-react';
import { api } from '@/lib/api';
import { Domain } from '@/types';
import { Button } from '@/components/ui/button';

export default function DomainsPage() {
  const router = useRouter();
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchDomains();
  }, []);

  const fetchDomains = async () => {
    try {
      const response = await api.getDomains();
      setDomains(response.domains || []);
    } catch (err) {
      setError('Failed to load domains');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to delete this domain?')) return;

    try {
      await api.deleteDomain(id);
      setDomains(domains.filter(d => d.id !== id));
    } catch (err) {
      setError('Failed to delete domain');
    }
  };

  if (loading) {
    return <div className="p-6">Loading...</div>;
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Domains</h1>
        <Button onClick={() => router.push('/dashboard/domains/create')}>
          <Plus className="w-4 h-4 mr-2" />
          Add Domain
        </Button>
      </div>

      {error && (
        <div className="bg-red-50 text-red-600 p-4 rounded mb-4">{error}</div>
      )}

      {domains.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          <Globe className="w-12 h-12 mx-auto mb-4 opacity-50" />
          <p>No custom domains yet</p>
          <p className="text-sm mt-2">Add a custom domain to use for your short URLs</p>
        </div>
      ) : (
        <div className="bg-white rounded-lg shadow">
          {domains.map((domain) => (
            <div
              key={domain.id}
              className="flex items-center justify-between p-4 border-b last:border-b-0"
            >
              <div>
                <p className="font-medium">{domain.domain}</p>
                <p className="text-sm text-gray-500">
                  Added {new Date(domain.created_at).toLocaleDateString()}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleDelete(domain.id)}
              >
                <Trash2 className="w-4 h-4 text-red-500" />
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
