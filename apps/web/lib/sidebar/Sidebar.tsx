"use client";

// Sidebar rewrite — PR-06. Project-grouped layout: each project
// has its own card with name + path + a session tree (placeholder
// for now: we don't yet have per-project session trees on the api,
// so we surface the project's session_count from the registry
// and the orphaned sessions in a separate "Other sessions"
// group). Active project is persisted in localStorage.

import { useEffect, useMemo, useState } from "react";
import { useT, useLocalizedPath } from "../i18n";
import { api } from "../api/client";
import {
  useProjects,
  type Project as RegistryProject,
  type OrphanSession,
} from "../projects/useProjects";
import { useCreateProjectDialog } from "../projects/CreateProjectDialogProvider";
import { BulkActionBar } from "./BulkActionBar";
interface ActiveSession {
  id: string;
  title: string;
  omp_cwd: string;
}

// The selected project lives in AgentShell (rh:selected-project-path);
// the sidebar is controlled through activeProjectPath/onSelectProject.
const COLLAPSE_KEY = "rh:sidebar-collapsed";
const TAB_KEY = "rh:right-sidebar-tab";

interface SidebarProps {
  activeId?: string;
  activeProjectPath?: string;
  onSelectProject?: (path: string) => void;
  onNewProject?: () => void;
}

export function Sidebar({
  activeId,
  activeProjectPath,
  onSelectProject,
  onNewProject,
}: SidebarProps) {
  const t = useT();
  const lp = useLocalizedPath();
  const dialog = useCreateProjectDialog();
  const { projects, orphans, loading } = useProjects(5000);
  const [sessions, setSessions] = useState<Record<string, ActiveSession[]>>({});
  const [sessionsLoaded, setSessionsLoaded] = useState<boolean>(false);
  const [collapsed, setCollapsed] = useState<boolean>(false);
  const activePath = activeProjectPath ?? null;
  // PR-07: bulk-select state. We keep an ordered Set (path ->
  // presence) so toggling and re-rendering is O(1). A plain array
  // would force dedup on every toggle.
  const [selectedPaths, setSelectedPaths] = useState<string[]>([]);

  // PR-02: shared "+ New project" opener. Prefer the prop override
  // (legacy callers), else the shared dialog context, else send
  // the user to /login (the old /agent/new gate is gone).
  const handleNewProject = onNewProject ?? (() => {
    if (dialog) {
      dialog.open();
    } else {
      window.location.href = lp("/login");
    }
  });

  // Read persisted state on mount.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const c = window.localStorage.getItem(COLLAPSE_KEY);
    if (c === "true") setCollapsed(true);
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(COLLAPSE_KEY, collapsed ? "true" : "false");
  }, [collapsed]);

  // Fetch live session list grouped by cwd. Cheap — registry
  // already gives us the list; we just need titles.
  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await api.get<{ groups: { omp_cwd: string; sessions: ActiveSession[] }[] }>(
          "/api/v1/sessions"
        );
        if (cancelled) return;
        const map: Record<string, ActiveSession[]> = {};
        for (const g of res.groups ?? []) {
          map[g.omp_cwd] = g.sessions;
        }
        setSessions(map);
        setSessionsLoaded(true);
      } catch {
        // ignore — registry poll will retry
      }
    }
    if (!collapsed) load();
    const id = setInterval(load, 5000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [collapsed]);

  const totalSessions = useMemo(
    () => Object.values(sessions).reduce((n, list) => n + list.length, 0) + orphans.length,
    [sessions, orphans]
  );

  // BUG 1 (PR-fix-batch-2): sort projects so the ones with live
  // sessions float to the top, empty projects sink to the bottom.
  // Without this the sidebar lists every project in registry
  // order and a long list of empty projects dominates the scroll.
  const visibleProjects = useMemo(() => {
    const empty: RegistryProject[] = [];
    const nonEmpty: RegistryProject[] = [];
    for (const p of projects) {
      if (p.hidden) continue;
      const list = sessions[p.path] ?? [];
      if (list.length > 0) nonEmpty.push(p);
      else empty.push(p);
    }
    return [...nonEmpty, ...empty];
  }, [projects, sessions]);

  // PR-07: bulk-select helpers. We keep selectedPaths as a plain
  // array (order is stable on insert; toggle re-derives membership).
  const visiblePaths = useMemo(
    () => projects.filter((p) => !p.hidden).map((p) => p.path),
    [projects],
  );
  const toggleSelected = (path: string) => {
    setSelectedPaths((cur) =>
      cur.includes(path) ? cur.filter((p) => p !== path) : [...cur, path],
    );
  };
  const clearSelected = () => setSelectedPaths([]);
  const toggleSelectAll = () => {
    setSelectedPaths((cur) =>
      cur.length === visiblePaths.length ? [] : visiblePaths,
    );
  };
  function persistTab(tab: string) {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(TAB_KEY, tab);
    }
  }
  if (collapsed) {
    return (
      <aside className="w-12 border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col items-center py-2 gap-2">
        <button
          type="button"
          onClick={() => setCollapsed(false)}
          className="rh-button-ghost px-2 py-1"
          aria-label={t("sidebar.expand")}
        >
          »
        </button>
        <button
          type="button"
          onClick={handleNewProject}
          className="rh-button-primary px-2 py-1"
          aria-label={t("projects.title")}
          title={t("projects.title")}
        >
          +
        </button>
      </aside>
    );
  }

  return (
    <aside className="w-64 border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col h-full">
      <header className="px-4 py-3 border-b border-[var(--color-border)] flex items-center justify-between">
        <h2 className="text-sm font-medium text-[var(--color-fg-muted)] uppercase tracking-wide">
          {t("sidebar.title")}
        </h2>
        <div className="flex gap-1">
          <button
            type="button"
            onClick={() => setCollapsed(true)}
            className="rh-button-ghost px-2 py-1 text-xs"
            aria-label={t("sidebar.collapse")}
            title={t("sidebar.collapse")}
          >
            «
          </button>
          <button
            type="button"
            onClick={handleNewProject}
            disabled={loading}
            className="rh-button-primary px-2 py-1 text-xs disabled:opacity-50"
          >
            + {t("projects.title")}
          </button>
        </div>
      </header>
      <div className="flex-1 overflow-y-auto px-2 py-2">
        {loading && projects.length === 0 ? (
          <p className="text-xs text-[var(--color-fg-subtle)] px-2">
            {t("sidebar.loading")}
          </p>
        ) : projects.length === 0 ? (
          <div className="flex flex-col items-center gap-2 px-3 py-6 text-center">
            <p className="text-xs text-[var(--color-fg-subtle)]">
              {t("projects.picker.firstTime")}
            </p>
            <button
              type="button"
              onClick={handleNewProject}
              className="text-xs rh-button-primary px-3 py-1"
            >
              {t("projects.title")}
            </button>
          </div>
        ) : (
          <ul className="flex flex-col gap-3">
            {!sessionsLoaded && (
              <li
                data-testid="sidebar-loading-sessions"
                className="px-2 py-1 text-xs text-[var(--color-fg-subtle)] italic"
              >
                {t("sidebar.loadingSessions")}
              </li>
            )}
            {visibleProjects.map((p) => (
              <ProjectCard
                key={p.path}
                project={p}
                sessions={sessions[p.path] ?? []}
                active={activePath === p.path}
                activeSessionId={activeId}
                selected={selectedPaths.includes(p.path)}
                onToggleSelect={() => toggleSelected(p.path)}
                onSelect={() => {
                  persistTab("files");
                  onSelectProject?.(p.path);
                }}
                onStartSession={async () => {
                  const res = await api.post<{ id: string }>("/api/v1/sessions", {
                    json: { omp_cwd: p.path, project_path: p.path },
                  });
                  if (res?.id) window.location.href = lp(`/agent/${res.id}`);
                }}
                onDeleteSession={async (id) => {
                  await api.delete(`/api/v1/sessions/${id}`);
                }}
                compact={(sessions[p.path] ?? []).length === 0}
              />
            ))}
            {orphans.length > 0 && (
              <OrphanGroup
                orphans={orphans}
                activeId={activeId}
                onStart={async (cwd) => {
                  const res = await api.post<{ id: string }>("/api/v1/sessions", {
                    json: { omp_cwd: cwd },
                  });
                  if (res?.id) window.location.href = lp(`/agent/${res.id}`);
                }}
              />
            )}
          </ul>
        )}
      </div>
      <footer className="px-4 py-2 border-t text-xs text-[var(--color-fg-subtle)] flex items-center justify-between">
        <span>{totalSessions}</span>
        <a href={lp("/")} className="hover:underline">
          {t("sidebar.title")}
        </a>
      </footer>
      {selectedPaths.length > 0 ? (
        <BulkActionBar
          selected={selectedPaths}
          allPaths={visiblePaths}
          onClear={clearSelected}
          onToggleSelectAll={toggleSelectAll}
        />
      ) : null}
    </aside>
  );
}

function ProjectCard({
  project,
  sessions,
  active,
  activeSessionId,
  selected,
  onToggleSelect,
  onSelect,
  onStartSession,
  onDeleteSession,
  compact = false,
}: {
  project: RegistryProject;
  sessions: ActiveSession[];
  active: boolean;
  activeSessionId?: string;
  selected: boolean;
  onToggleSelect: () => void;
  onSelect: () => void;
  onStartSession: () => void;
  onDeleteSession: (id: string) => void;
  compact?: boolean;
}) {
  const t = useT();
  const lp = useLocalizedPath();
  return (
    <li
      data-active={active}
      data-compact={compact}
      className={`flex flex-col ${compact ? "gap-0 px-2 py-1" : "gap-1 px-2 py-2"} rounded border-l-2 ${
        active
          ? "border-blue-500 bg-[var(--color-bg-card)]"
          : "border-transparent"
      }`}
    >
      <div className="flex items-start gap-2">
        <input
          type="checkbox"
          className="mt-1"
          checked={selected}
          onChange={onToggleSelect}
          aria-label={t("projects.bulk.selectAll") + " " + project.name}
          onClick={(e) => e.stopPropagation()}
        />
        <button
          type="button"
          onClick={onSelect}
          className="text-left flex-1"
        >
          <div className="flex items-center justify-between gap-2">
            <span className="font-medium text-sm truncate">{project.name}</span>
            <span className="text-xs text-[var(--color-fg-muted)]">
              {project.session_count}
            </span>
          </div>
          <div className="text-xs text-[var(--color-fg-muted)] font-mono truncate">
            {project.path}
          </div>
        </button>
      </div>
      <div className="flex flex-col gap-0.5">
        {sessions.length === 0 ? (
          <button
            type="button"
            onClick={onStartSession}
            className="text-xs rh-button-ghost px-2 py-1 self-start"
          >
            + {t("agent.newSession")}
          </button>
        ) : (
          sessions.map((s) => (
            <div
              key={s.id}
              className={`flex items-center gap-2 px-2 py-1 rounded text-xs ${
                s.id === activeSessionId
                  ? "bg-[var(--color-bg-card)]"
                  : "hover:bg-[var(--color-bg-card)]"
              }`}
            >
              <a href={lp(`/agent/${s.id}`)} className="flex-1 truncate">
                {s.title || s.id}
              </a>
              <button
                type="button"
                onClick={() => onDeleteSession(s.id)}
                aria-label={t("sidebar.delete")}
                className="text-[var(--color-fg-subtle)] hover:text-[var(--color-danger)]"
              >
                ×
              </button>
            </div>
          ))
        )}
      </div>
    </li>
  );
}

function OrphanGroup({
  orphans,
  activeId,
  onStart,
}: {
  orphans: OrphanSession[];
  activeId?: string;
  onStart: (cwd: string) => void;
}) {
  const t = useT();
  const lp = useLocalizedPath();
  const [expanded, setExpanded] = useState(true);
  return (
    <li className="flex flex-col gap-1 px-2 py-2 rounded bg-[var(--color-bg-card)] border border-dashed border-[var(--color-border)]">
      <button
        type="button"
        onClick={() => setExpanded((e) => !e)}
        className="flex items-center justify-between text-xs text-[var(--color-fg-muted)]"
      >
        <span>{t("sidebar.others")}</span>
        <span>
          {expanded ? "▾" : "▸"} {orphans.length}
        </span>
      </button>
      {expanded && (
        <ul className="flex flex-col gap-0.5">
          {orphans.map((o) => (
            <li
              key={o.id}
              className={`flex items-center justify-between gap-2 px-2 py-1 rounded text-xs ${
                o.id === activeId
                  ? "bg-[var(--color-bg-elevated)]"
                  : "hover:bg-[var(--color-bg-elevated)]"
              }`}
            >
              <a
                href={lp(`/agent/${o.id}`)}
                className="flex-1 truncate font-mono"
                title={o.omp_cwd}
              >
                {o.omp_cwd.split("/").filter(Boolean).pop() || o.omp_cwd}
              </a>
              <button
                type="button"
                onClick={() => onStart(o.omp_cwd)}
                className="text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]"
                aria-label={t("agent.newSession")}
              >
                +
              </button>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}
