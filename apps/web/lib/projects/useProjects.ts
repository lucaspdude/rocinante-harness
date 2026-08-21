"use client";

// useProjects — polls /api/v1/projects every 5s. Same shape as the
// reference (lucaspdude/ompweb). PR-05 hooks this up to the
// CreateProjectDialog + new-session flow.

import { useCallback, useEffect, useState } from "react";
import { api } from "../api/client";

export interface Project {
  path: string;
  name: string;
  description?: string;
  added_at: string;
  updated_at: string;
  hidden?: boolean;
  session_count: number;
}

export interface OrphanSession {
  id: string;
  omp_cwd: string;
}

interface ProjectsListResponse {
  projects: Project[];
  orphans?: OrphanSession[];
}

export interface CreateProjectInput {
  path: string;
  name?: string;
  description?: string;
}

export interface CloneProjectInput {
  url: string;
  parent_path: string;
  folder_name?: string;
}

export interface CloneStartResponse {
  job_id: string;
  stream_url: string;
  status_url: string;
  url: string;
  target: string;
}
export interface BulkProjectResult {
  archived?: number;
  deleted?: number;
  errors?: Array<{ path: string; code: string; message?: string }>;
}

export function useProjects(intervalMs = 5000, enabled = true): {
  projects: Project[];
  orphans: OrphanSession[];
  loading: boolean;
  error: string | null;
  reload: () => void;
  register: (input: CreateProjectInput) => Promise<Project>;
  patch: (path: string, name?: string, description?: string) => Promise<Project>;
  hide: (path: string, hidden?: boolean) => Promise<void>;
  bulkArchive: (paths: string[]) => Promise<BulkProjectResult>;
  bulkDelete: (paths: string[], confirmPath: string) => Promise<BulkProjectResult>;
  startClone: (input: CloneProjectInput) => Promise<CloneStartResponse>;
} {
  const [projects, setProjects] = useState<Project[]>([]);
  const [orphans, setOrphans] = useState<OrphanSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    async function load() {
      try {
        const res = await api.get<ProjectsListResponse>("/api/v1/projects");
        if (cancelled) return;
        setProjects(res.projects ?? []);
        setOrphans(res.orphans ?? []);
        setError(null);
      } catch (e: unknown) {
        if (!cancelled) {
          const err = e as { body?: { message?: string }; message?: string };
          setError(err.body?.message ?? err.message ?? "failed");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    if (intervalMs <= 0) return () => { cancelled = true; };
    const id = setInterval(load, intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [intervalMs, enabled, tick]);
  useEffect(() => {
    if (typeof window === "undefined") return;
    function onCreated() { setTick((n) => n + 1); }
    window.addEventListener("rh:project:created", onCreated);
    return () => window.removeEventListener("rh:project:created", onCreated);
  }, []);


  const reload = useCallback(() => setTick((n) => n + 1), []);

  const register = useCallback(
    async (input: CreateProjectInput) => {
      const created = await api.post<Project>("/api/v1/projects", { json: input });
      setProjects((cur) => {
        const next = [...cur.filter((p) => p.path !== created.path), created];
        next.sort((a, b) => a.path.localeCompare(b.path));
        return next;
      });
      reload();
      return created;
    },
    [reload]
  );

  const patch = useCallback(
    async (path: string, name?: string, description?: string) => {
      const body: { path: string; name?: string; description?: string } = { path };
      if (name !== undefined) body.name = name;
      if (description !== undefined) body.description = description;
      const updated = await api.patch<Project>("/api/v1/projects", { json: body });
      setProjects((cur) =>
        cur.map((p) => (p.path === updated.path ? updated : p))
      );
      return updated;
    },
    []
  );

  const hide = useCallback(
    async (path: string, hidden = true) => {
      await api.delete("/api/v1/projects", {
        json: { path, hidden },
      });
      setProjects((cur) =>
        cur.map((p) =>
          p.path === path ? { ...p, hidden } : p
        )
      );
    },
    []
  );

  const bulkArchive = useCallback(
    async (paths: string[]): Promise<BulkProjectResult> => {
      const res = await api.post<BulkProjectResult>("/api/v1/projects/bulk", {
        json: { op: "archive", paths },
      });
      // Optimistically hide successful entries; reload reconciles.
      setProjects((cur) => {
        const failed = new Set(
          (res.errors ?? []).map((e) => e.path),
        );
        return cur.map((p) =>
          paths.includes(p.path) && !failed.has(p.path)
            ? { ...p, hidden: true }
            : p
        );
      });
      reload();
      return res;
    },
    [reload]
  );

  const bulkDelete = useCallback(
    async (paths: string[], confirmPath: string): Promise<BulkProjectResult> => {
      const res = await api.post<BulkProjectResult>("/api/v1/projects/bulk", {
        json: { op: "delete", paths, confirmPath },
      });
      // Optimistically mark deleted entries hidden; the api already
      // wiped the on-disk directory and Hide()'d the registry entry.
      setProjects((cur) => {
        const failed = new Set(
          (res.errors ?? []).map((e) => e.path),
        );
        return cur.map((p) =>
          paths.includes(p.path) && !failed.has(p.path)
            ? { ...p, hidden: true }
            : p
        );
      });
      reload();
      return res;
    },
    [reload]
  );

  const startClone = useCallback(async (input: CloneProjectInput) => {
    return api.post<CloneStartResponse>("/api/v1/projects/clone", {
      json: input,
    });
  }, []);

  return {
    projects,
    orphans,
    loading,
    error,
    reload,
    register,
    patch,
    hide,
    bulkArchive,
    bulkDelete,
    startClone,
  };
}
