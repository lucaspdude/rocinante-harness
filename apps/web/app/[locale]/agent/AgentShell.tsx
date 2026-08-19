"use client";

// PR-02: client-side shell for /agent/*.
//
// Mounts the shared CreateProjectDialogProvider around Sidebar + the
// page content, so both the Sidebar's "+ New project" buttons and the
// top-bar (ProjectSelectorBar) call the same opener. activeId is read
// from the URL via useParams(); the /agent and /agent/new segments
// get an empty activeId so the sidebar highlights nothing.

import type { ReactNode } from "react";
import { useParams } from "next/navigation";
import { Sidebar } from "../../../lib/sidebar/Sidebar";
import { RightSidebar } from "../../../lib/sidebar/RightSidebar";
import { CreateProjectDialogProvider } from "../../../lib/projects/CreateProjectDialogProvider";

export function AgentShell({ children }: { children: ReactNode }) {
  const params = useParams<{ id?: string }>();
  const activeId = params?.id ?? "";
  return (
    <CreateProjectDialogProvider>
      <div className="flex flex-1 min-h-0">
        <Sidebar activeId={activeId} />
        <div className="flex-1 flex flex-col min-w-0">{children}</div>
        <RightSidebar cwd={null} />
      </div>
    </CreateProjectDialogProvider>
  );
}
