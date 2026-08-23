import { defineConfig } from "vitest/config";

export default defineConfig({
  esbuild: {
    jsx: "automatic",
  },
  test: {
    exclude: ["**/node_modules/**", "**/dist/**", "e2e/**"],
    // Phase 7: use jsdom for tests that render React (e.g. the
    // useAuthStatus hook test). Pure-logic tests under
    // tests/*.test.ts and lib/*/*.test.ts run in node by default;
    // those files should NOT add the `// @vitest-environment jsdom`
    // comment because the project convention is "node unless
    // React rendering is required".
    environment: "node",
    environmentMatchGlobs: [
      ["lib/auth/auth-status.test.ts", "jsdom"],
    ],
  },
});
