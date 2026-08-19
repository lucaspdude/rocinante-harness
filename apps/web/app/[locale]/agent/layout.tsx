// PR-02: parent layout for the entire /agent/* tree. Lifts the
// TopNav + Sidebar + RightSidebar + CreateProjectDialogProvider
// shell so all of /agent, /agent/new, and /agent/[id] inherit the
// same chrome and share the single CreateProjectDialog instance.
//
// This replaces the per-segment layouts that lived in
// /agent/[id]/layout.tsx and /agent/new/layout.tsx. Those are
// deleted as part of this PR.

import type { ReactNode } from "react";
import { TopNav } from "../../../lib/components/TopNav";
import { AgentShell } from "./AgentShell";

export default function AgentLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-screen flex-col">
      <TopNav />
      <AgentShell>{children}</AgentShell>
    </div>
  );
}
