import type { NextConfig } from "next";

const apiUrl = process.env.API_URL ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  deploymentId: process.env.ILLARIN_VERSION,
  logging: {
    incomingRequests: {
      // These URLs carry short-lived link secrets. nginx redacts them too.
      ignore: [
        /^\/link(?:\?|$)/,
        /^\/api\/v1\/link\/requests\/[^/]+/,
        /^\/api\/v1\/link\/authorizations\/[^/]+/,
      ],
    },
  },
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          {
            key: "Content-Security-Policy",
            value: "frame-ancestors 'none'",
          },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Referrer-Policy", value: "no-referrer" },
        ],
      },
    ];
  },
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${apiUrl}/:path*`,
      },
    ];
  },
};

export default nextConfig;
