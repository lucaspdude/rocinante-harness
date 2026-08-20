"use client";

// PR-09: global keyboard shortcuts.
//
// Mounts a single document-level keydown listener that turns the three
// high-frequency shortcuts (Cmd+Enter send, Esc cancel, Cmd+K palette)
// into CustomEvents on `window`. Each feature listens for the event it
// cares about — the composer for `rh:composer-send`, every modal for
// `rh:close-top-modal`. The palette event is dispatched but has no
// consumer yet; future PRs will wire the command palette UI.
//
// Why CustomEvents instead of inline `e.preventDefault()` + feature
// call here:
//   - Keeps this hook independent of any specific feature module.
//   - Avoids double-firing when a feature already has a local handler
//     (the event simply has no listener and is a no-op).
//   - Lets future shortcuts slot in without touching this file.
//
// Scoping of preventDefault:
//   - Cmd+Enter: always preventDefault. Cmd+Enter has no browser
//     default in a textarea, but if focus is on a button the browser
//     may activate it; preventDefault guards against that.
//   - Escape: never preventDefault. Browser fullscreen exit and IME
//     cancellation rely on the default behavior; each modal handles
//     its own close via the event.
//   - Cmd+K: preventDefault only when focus is NOT inside an
//     input/textarea/contentEditable. This matches the gate already
//     used by ProjectSelectorBar's local Cmd+K toggle so neither hook
//     swallows the user's typing.

import { useEffect } from "react";

export const RH_COMPOSER_SEND = "rh:composer-send";
export const RH_CLOSE_TOP_MODAL = "rh:close-top-modal";
export const RH_OPEN_COMMAND_PALETTE = "rh:open-command-palette";

function isTextEditingTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  const tag = el.tagName?.toLowerCase();
  return tag === "input" || tag === "textarea" || el.isContentEditable;
}

function handleShortcut(e: KeyboardEvent): void {
  const isMod = e.metaKey || e.ctrlKey;

  if (isMod && e.key === "Enter") {
    e.preventDefault();
    window.dispatchEvent(new CustomEvent(RH_COMPOSER_SEND));
    return;
  }

  if (e.key === "Escape") {
    // Don't preventDefault — let browser exit fullscreen, IME
    // cancellation, etc. Modals handle their own close logic via
    // the dispatched event (or via their local handlers).
    window.dispatchEvent(new CustomEvent(RH_CLOSE_TOP_MODAL));
    return;
  }

  if (isMod && e.key.toLowerCase() === "k") {
    if (isTextEditingTarget(e.target)) return;
    e.preventDefault();
    window.dispatchEvent(new CustomEvent(RH_OPEN_COMMAND_PALETTE));
    return;
  }
}

export function useShortcuts(): void {
  useEffect(() => {
    document.addEventListener("keydown", handleShortcut);
    return () => document.removeEventListener("keydown", handleShortcut);
  }, []);
}
