"use client";

// PR-05: New-session flow rewired through the project registry.
// Logic:
//   1. Poll /api/v1/projects (useProjects hook).
//   2. If empty, open CreateProjectDialog directly.
//   3. Otherwise show ProjectPickerDialog; selected project becomes
//      the session's omp_cwd.

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useT, useLocalizedPath } from "../../../../lib/i18n";
import { api } from "../../../../lib/api/client";
import { useProjects, type Project } from "../../../../lib/projects/useProjects";
import {
  CreateProjectDialog,
  type CreateProjectDialogProps,
} from "../../../../lib/projects/CreateProjectDialog";
import { ProjectPickerDialog } from "../../../../lib/projects/ProjectPickerDialog";

interface SessionRecord {
  id: string;
}

export default function NewSessionPage() {
  const t = useT();
  const router = useRouter();
  const lp = useLocalizedPath();
  const { projects, loading } = useProjects(3000);
  const [phase, setPhase] = useState<"loading" | "no-projects" | "picker" | "creating">("loading");
  const [createOpen, setCreateOpen] = useState(false);
  const [createTab, setCreateTab] = useState<CreateProjectDialogProps["initialTab"]>("folder");

  useEffect(() => {
    if (loading) return;
    if (projects.length === 0) {
      setPhase("no-projects");
      setCreateTab("folder");
      setCreateOpen(true);
    } else {
      setPhase("picker");
    }
  }, [loading, projects]);

  async function startSession(p: Project) {
    setPhase("creating");
    try {
      const session = await api.post<SessionRecord>("/api/v1/sessions", {
        json: { omp_cwd: p.path, project_path: p.path },
      });
      if (session?.id) {
        router.replace(lp(`/agent/${session.id}`));
      } else {
        router.replace(lp("/"));
      }
    } catch {
      router.replace(lp("/"));
    }
  }

  return (
    <main className="max-w-2xl mx-auto px-4 py-16">
      <div className="rh-card">
        <p className="text-[var(--color-fg-muted)]">
          {phase === "loading" && t("agent.connecting")}
          {phase === "no-projects" && t("projects.picker.firstTime")}
          {phase === "picker" && t("projects.picker.chooseExisting")}
          {phase === "creating" && t("agent.connecting")}
        </p>
      </div>
      <CreateProjectDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(p) => {
          setCreateOpen(false);
          // After creation, the next poll picks the new project up;
          // for now optimistically treat as success and navigate.
          if (phase === "creating") return;
          startSession({
            path: p,
            name: p.split("/").filter(Boolean).pop() ?? p,
            added_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
            session_count: 0,
          });
        }}
        initialTab={createTab}
      />
      <ProjectPickerDialog
        open={phase === "picker"}
        onClose={() => router.replace(lp("/"))}
        onPick={(p) => startSession(p)}
      />
    </main>
  );
}
