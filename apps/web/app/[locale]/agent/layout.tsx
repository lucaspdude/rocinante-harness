"use client";

// PR-02: parent layout for the entire /agent/* tree. Lifts the
// TopNav + Sidebar + RightSidebar + CreateProjectDialogProvider
// shell so all of /agent, /agent/new, and /agent/[id] inherit the
// same chrome and share the single CreateProjectDialog instance.
//
// This replaces the per-segment layouts that lived in
// /agent/[id]/layout.tsx and /agent/new/layout.tsx. Those are
// deleted as part of this PR.
//
// Phase 7 — item 03: gate children on auth. The chrome (TopNav
// + AgentShell with its Sidebar/ProjectSelectorBar) is rendered
// ALWAYS so the user sees a consistent surface during the
// loading window. The redirect fires from useEffect (not the
// render body) so React anti-patterns are avoided. While
// loading, the <main> area shows a centered spinner.

import { useEffect, useState, type ReactNode } from "react";
import { useT, useLocalizedPath } from "../../../lib/i18n";
import { useAuthStatus } from "../../../lib/auth/auth-status";
import { tokenStore } from "../../../lib/auth/token-store";
import { TopNav } from "../../../lib/components/TopNav";
import { AgentShell } from "./AgentShell";

export default function AgentLayout({ children }: { children: ReactNode }) {
  const t = useT();
  const lp = useLocalizedPath();
  const hasToken = tokenStore.peek();
  const { loading, status } = useAuthStatus();
  const [decided, setDecided] = useState(false);

  useEffect(() => {
    if (loading) return;
    if (hasToken) {
      setDecided(true);
      return;
    }
    if (status && !status.auth_required) {
      // auth disabled (api not initialized). Allow the agent
      // shell to mount — there is nothing to gate.
      setDecided(true);
      return;
    }
    const next = window.location.pathname + window.location.search;
    window.location.href = `${lp("/login")}?next=${encodeURIComponent(next)}`;
  }, [loading, hasToken, status, lp]);

  // Wrap children in a fragment-style gate: when not decided,
  // render the loading hint instead. The wrapper <main>-style
  // div in AgentShell handles layout — we only swap the
  // children that go inside the <main> area.
  const gatedChildren = !decided ? (
    <div className="flex h-full items-center justify-center text-sm text-[var(--color-fg-muted)]">
      {t("common.loading")}
    </div>
  ) : (
    children
  );

  return (
    <div className="flex h-screen flex-col">
      <TopNav />
      <AgentShell>{gatedChildren}</AgentShell>
    </div>
  );
}
