import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'standalone',
  async redirects() {
    return [
      // The old stats page is now part of the link details page.
      { source: '/dashboard/links/:id/stats', destination: '/dashboard/links/:id', permanent: false },
    ];
  },
};

export default nextConfig;
