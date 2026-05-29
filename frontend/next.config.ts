import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Enable standalone output for Docker production builds
  output: "standalone",

  // Run dev server on port 3001
  env: {
    NEXT_PUBLIC_BACKEND_WS_PORT: "8080",
  },
};

export default nextConfig;
