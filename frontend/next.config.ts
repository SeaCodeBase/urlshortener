import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'standalone',
  async redirects() {
    return [
      // The old stats page is now part of the link details page.
      { source: '/dashboard/links/:id/stats', destination: '/dashboard/links/:id', permanent: false },
      // Domains moved to the Bitly-style "Custom domains" page.
      { source: '/dashboard/domains', destination: '/dashboard/custom-domains', permanent: false },
      { source: '/dashboard/domains/:path*', destination: '/dashboard/custom-domains', permanent: false },
    ];
  },
};

export default nextConfig;
