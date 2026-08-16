import type { NextConfig } from "next";

const config: NextConfig = {
  reactStrictMode: true,
  output: "standalone",
  // The web talks to the api via a same-origin rewrite. The
  // browser only ever hits the web on :30178; the rewrite proxies
  // /api/v1/* to the api on loopback. No CORS, no public API URL
  // env var, no NEXT_PUBLIC_RH_API_URL to remember — the same
  // bundle works on localhost, a LAN IP, or a public hostname.
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: "http://127.0.0.1:30179/api/v1/:path*",
      },
    ];
  },
};

export default config;
