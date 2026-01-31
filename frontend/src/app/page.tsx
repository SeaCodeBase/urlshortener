'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { useAuthStore } from '@/stores/auth';

export default function Home() {
  const router = useRouter();
  const { user, isLoading, checkAuth } = useAuthStore();

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    if (!isLoading && user) {
      router.push('/dashboard');
    }
  }, [isLoading, user, router]);

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        Loading...
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50">
      <main className="flex flex-col items-center gap-8 text-center px-4">
        <h1 className="text-4xl font-bold tracking-tight text-gray-900 sm:text-6xl">
          URL Shortener
        </h1>
        <p className="text-lg text-gray-600 max-w-md">
          Create short, memorable links in seconds. Track clicks and manage your URLs with ease.
        </p>
        <div className="flex gap-4">
          <Link href="/login">
            <Button size="lg">
              Get Started
            </Button>
          </Link>
        </div>
      </main>
    </div>
  );
}
