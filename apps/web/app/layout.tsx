import type { ReactNode } from "react";
import { THEME_BOOTSTRAP_SCRIPT } from "../lib/theme/useTheme";
import "./globals.css";

export const metadata = {
  title: "rocinante-harness",
  description: "AI agent product",
};

// Inline theme bootstrap runs before React hydrates so the palette
// is correct on first paint. See apps/web/lib/theme/useTheme.ts
// THEME_BOOTSTRAP_SCRIPT for the source. Mirrors the harness
// reference pattern (docs/ui-ux-references/desktop.md §1) — a
// single `document.documentElement.style.colorScheme = ...`
// attribute flip drives dark / light without re-renders.
//
// THEME_BOOTSTRAP_SCRIPT is already an IIFE string; we drop it
// straight into a <script> tag.
export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        <script dangerouslySetInnerHTML={{ __html: THEME_BOOTSTRAP_SCRIPT }} />
      </head>
      <body>{children}</body>
    </html>
  );
}