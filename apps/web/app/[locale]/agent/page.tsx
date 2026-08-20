"use client";

// PR-02: chat-first home for the agent area. The previous /agent/new
// gate is gone — this is the new entry point. The ProjectSelectorBar
// lives in AgentShell (shared with the sidebar so both highlight the
// same project); this page renders only the session-less ChatComposer.

import { useProjects, type Project } from "../../../lib/projects/useProjects";
import { ChatComposer } from "../../../lib/agent/ChatComposer";
import { useSelectedProject } from "./AgentShell";

export default function AgentHomePage() {
  const { projects } = useProjects(5000);
  const selection = useSelectedProject();
  const selectedPath = selection?.selectedPath ?? null;

  const project: Project | null = selectedPath
    ? projects.find((p) => p.path === selectedPath) ?? null
    : null;

  return (
    <div className="flex flex-col h-full min-h-0">
      <div className="flex-1 flex flex-col justify-center min-h-0 overflow-y-auto">
        <ChatComposer project={project} />
      </div>
    </div>
  );
}
