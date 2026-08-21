"use client";

// PR-09: client-only mount point for the global keyboard shortcut
// hook. Renders nothing — its sole job is to call useShortcuts() once
// at the application root so a single document-level listener handles
// every shortcut. Separating it from layout.tsx (a server component)
// keeps the rest of the tree free to mix server and client.

import { useShortcuts } from "./useShortcuts";

export function KeyboardShortcutsRoot(): null {
  useShortcuts();
  return null;
}
