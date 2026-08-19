"use client";

// PR-02: chat-first home for the agent area. The previous /agent/new
// gate is gone — this is the new entry point. A ProjectSelectorBar
// sits at the top, and the session-less ChatComposer fills the rest.
// No modal auto-opens; the user must click "+ New project" in the top
// bar to register their first project.

import { useEffect, useState } from "react";
import { ProjectSelectorBar } from "../../../lib/projects/ProjectSelectorBar";
import { useProjects, type Project } from "../../../lib/projects/useProjects";
import { ChatComposer } from "../../../lib/agent/ChatComposer";

const SELECTED_KEY = "rh:selected-project-path";

export default function AgentHomePage() {
  const { projects, loading } = useProjects(5000);
  const [selectedPath, setSelectedPath] = useState<string | null>(null);

  // Hydrate from localStorage on mount.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const stored = window.localStorage.getItem(SELECTED_KEY);
    if (stored) setSelectedPath(stored);
  }, []);

  // Persist on change.
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (selectedPath) {
      window.localStorage.setItem(SELECTED_KEY, selectedPath);
    } else {
      window.localStorage.removeItem(SELECTED_KEY);
    }
  }, [selectedPath]);

  // If the stored path no longer matches any registered project
  // (user deleted it on disk or renamed it), fall back to no
  // selection. The PR spec calls this "fall back to null".
  useEffect(() => {
    if (!selectedPath) return;
    if (loading && projects.length === 0) return;
    if (!projects.some((p) => p.path === selectedPath)) {
      setSelectedPath(null);
    }
  }, [projects, selectedPath, loading]);

  // After a CreateProjectDialog creation, the registry hook's poll
  // will pick the project up within 5s. The provider also dispatches
  // `rh:project:created` so we can auto-select the fresh entry
  // immediately.
  useEffect(() => {
    function onCreated(e: Event) {
      const path = (e as CustomEvent<string>).detail;
      if (typeof path === "string" && path.length > 0) {
        setSelectedPath(path);
      }
    }
    window.addEventListener("rh:project:created", onCreated);
    return () => window.removeEventListener("rh:project:created", onCreated);
  }, []);

  const project: Project | null = selectedPath
    ? projects.find((p) => p.path === selectedPath) ?? null
    : null;

  return (
    <div className="flex flex-col h-full min-h-0">
      <ProjectSelectorBar
        projects={projects}
        loading={loading}
        error={null}
        selectedPath={selectedPath}
        onSelect={setSelectedPath}
      />
      <div className="flex-1 flex flex-col justify-center min-h-0 overflow-y-auto">
        <ChatComposer project={project} />
      </div>
    </div>
  );
}
