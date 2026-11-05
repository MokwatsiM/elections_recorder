import type { NextConfig } from "next";

const backend = process.env.BACKEND_URL ?? "http://localhost:8080";

const config: NextConfig = {
  output: "standalone",
  distDir: process.env.NEXT_DIST_DIR || ".next",
  // Compression would buffer the SSE stream. Caddy compresses everything else.
  compress: false,
  async rewrites() {
    return [
      { source: "/api/v1/:path*", destination: `${backend}/api/v1/:path*` },
      { source: "/tiles/:path*", destination: `${backend}/tiles/:path*` },
    ];
  },
};
export default config;
