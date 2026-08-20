"use client";

// PR-02: client-side shell for /agent/*.
//
// Mounts the shared CreateProjectDialogProvider around Sidebar + the
// page content, so both the Sidebar's "+ New project" buttons and the
// top-bar (ProjectSelectorBar) call the same opener. activeId is read
// from the URL via useParams(); the /agent and /agent/new segments
// get an empty activeId so the sidebar highlights nothing.
//
// The shell also owns the selected-project path. Sidebar and
// ProjectSelectorBar are both controlled by it, so selecting a project
// in either surface highlights it in the other. The path is persisted
// under rh:selected-project-path; the legacy rh:active-project-path key
// written by older builds is migrated once on mount.

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useParams } from "next/navigation";
import { Sidebar } from "../../../lib/sidebar/Sidebar";
import { RightSidebar } from "../../../lib/sidebar/RightSidebar";
import { CreateProjectDialogProvider } from "../../../lib/projects/CreateProjectDialogProvider";
import { ProjectSelectorBar } from "../../../lib/projects/ProjectSelectorBar";
import { useProjects } from "../../../lib/projects/useProjects";

const SELECTED_KEY = "rh:selected-project-path";
const LEGACY_KEY = "rh:active-project-path";

interface SelectedProjectCtx {
  selectedPath: string | null;
  setSelectedPath: (path: string | null) => void;
}

// null when a page renders outside the agent shell (tests). Consumers
// should null-check and fall back to their own state.
const CtxImpl = createContext<SelectedProjectCtx | null>(null);

export function useSelectedProject(): SelectedProjectCtx | null {
  return useContext(CtxImpl);
}

export function AgentShell({ children }: { children: ReactNode }) {
  const params = useParams<{ id?: string }>();
  const activeId = params?.id ?? "";
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const { projects, loading } = useProjects(5000);

  // Hydrate on mount, migrating the legacy key written by builds that
  // kept the sidebar's selection separate from the top bar's.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const stored = window.localStorage.getItem(SELECTED_KEY);
    if (stored) {
      setSelectedPath(stored);
      window.localStorage.removeItem(LEGACY_KEY);
      return;
    }
    const legacy = window.localStorage.getItem(LEGACY_KEY);
    if (legacy) {
      window.localStorage.setItem(SELECTED_KEY, legacy);
      window.localStorage.removeItem(LEGACY_KEY);
      setSelectedPath(legacy);
    }
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

  // If the stored path no longer matches any registered project (the
  // user deleted or renamed it on disk), fall back to no selection.
  useEffect(() => {
    if (!selectedPath) return;
    if (loading && projects.length === 0) return;
    if (!projects.some((p) => p.path === selectedPath)) {
      setSelectedPath(null);
    }
  }, [projects, selectedPath, loading]);

  // The CreateProjectDialogProvider dispatches rh:project:created after
  // a successful registration so the fresh project becomes selected
  // without waiting for the 5s registry poll.
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

  const ctx = useMemo<SelectedProjectCtx>(
    () => ({ selectedPath, setSelectedPath }),
    [selectedPath],
  );

  return (
    <CreateProjectDialogProvider>
      <CtxImpl.Provider value={ctx}>
        <ProjectSelectorBar
          projects={projects}
          loading={loading}
          error={null}
          selectedPath={selectedPath}
          onSelect={setSelectedPath}
        />
        <div className="flex flex-1 min-h-0">
          <Sidebar
            activeId={activeId}
            activeProjectPath={selectedPath ?? undefined}
            onSelectProject={setSelectedPath}
          />
          <div className="flex-1 flex flex-col min-w-0">{children}</div>
          <RightSidebar cwd={null} />
        </div>
      </CtxImpl.Provider>
    </CreateProjectDialogProvider>
  );
}
