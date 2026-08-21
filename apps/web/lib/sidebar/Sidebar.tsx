"use client";

// Sidebar — PR-04 rewrite. Two-layer workspace tree (workspaces + their
// sessions) styled after the DeepSeek reference captured in
// docs/ui-ux-references/desktop.md §2/§4. The old version was 13 KB /
// 430 LOC of mixed concerns (project card, orphan group, session list,
// bulk-action bar, +/- buttons scattered around). The new version is
// built around two composable primitives:
//
//   <WorkspaceRow> — chevron + folder + name + ellipsis-on-hover; click
//                     to expand the session list nested below it.
//   <SessionRow>    — bullet + title + relative time + ellipsis-on-hover;
//                     click navigates to /agent/{id}.
//
// The sidebar itself can be collapsed to a 56 px icon rail. The state
// lives in localStorage under "rh:sidebar-collapsed" so the choice
// survives reloads. Mobile (< 768 px) defaults to collapsed and exposes
// a single tap target so the user can re-open it inline.
//
// The BulkActionBar from phase-3 still floats over the bottom of the
// sidebar when 2+ projects are selected — its props are unchanged.

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
} from "react";
import {
  ChevronDown,
  ChevronRight,
  Copy,
  EyeOff,
  Folder,
  FolderOpen,
  MoreHorizontal,
  PanelLeftClose,
  PanelLeftOpen,
  Pencil,
  Plus,
  Search,
  Settings,
  Sparkles,
  Trash2,
  X,
} from "lucide-react";
import { useT, useLocalizedPath } from "../i18n";
import { api } from "../api/client";
import { useToast } from "../toast";
import {
  useProjects,
  type Project as RegistryProject,
  type OrphanSession,
} from "../projects/useProjects";
import { useCreateProjectDialog } from "../projects/CreateProjectDialogProvider";
import { BulkActionBar } from "./BulkActionBar";

// --- Persistence keys --------------------------------------------------

const COLLAPSE_KEY = "rh:sidebar-collapsed";
const SEARCH_KEY = "rh:sidebar-search";
const TAB_KEY = "rh:right-sidebar-tab";

// --- Mobile breakpoint -------------------------------------------------

const MOBILE_QUERY = "(max-width: 767px)";

// --- Public types ------------------------------------------------------

interface ActiveSession {
  id: string;
  title: string;
  omp_cwd: string;
  updated_at?: string;
  created_at?: string;
}

interface SidebarProps {
  activeId?: string;
  activeProjectPath?: string;
  onSelectProject?: (path: string) => void;
  onNewProject?: () => void;
}

// --- Helpers -----------------------------------------------------------

function relativeTime(
  when: string | undefined,
  t: (key: string, vars?: Record<string, string | number>) => string,
): string {
  if (!when) return t("sidebar.project.lastActivity.never");
  const ts = Date.parse(when);
  if (Number.isNaN(ts)) return t("sidebar.project.lastActivity.never");
  const diffSec = (Date.now() - ts) / 1000;
  if (diffSec < 60) return t("sidebar.project.lastActivity.justNow");
  if (diffSec < 3600) {
    const m = Math.floor(diffSec / 60);
    return t("sidebar.project.lastActivity.minutesAgo", { count: m });
  }
  if (diffSec < 86400) {
    const h = Math.floor(diffSec / 3600);
    return t("sidebar.project.lastActivity.hoursAgo", { count: h });
  }
  const d = Math.floor(diffSec / 86400);
  return t("sidebar.project.lastActivity.daysAgo", { count: d });
}

function basename(p: string): string {
  const trimmed = p.replace(/\/+$/, "");
  if (!trimmed) return p;
  const slash = trimmed.lastIndexOf("/");
  return slash === -1 ? trimmed : trimmed.slice(slash + 1);
}

function matchesSearch(haystack: string, needle: string): boolean {
  return haystack.toLowerCase().includes(needle.toLowerCase());
}

// --- Sidebar ------------------------------------------------------------

export function Sidebar({
  activeId,
  activeProjectPath,
  onSelectProject,
  onNewProject,
}: SidebarProps) {
  const t = useT();
  const lp = useLocalizedPath();
  const dialog = useCreateProjectDialog();
  const toast = useToast();
  const { projects, orphans, loading, patch, hide } = useProjects(5000);

  // Shared collapse state. Auto-collapses on mobile (< 768 px) per the
  // acceptance criteria; the user can override and we persist whatever
  // they choose.
  const [collapsed, setCollapsed] = useState<boolean>(false);
  const [mobile, setMobile] = useState<boolean>(false);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [search, setSearch] = useState<string>("");
  const [sessions, setSessions] = useState<Record<string, ActiveSession[]>>({});
  // PR-07: bulk-select state. Plain array for stable ordering; toggle
  // re-derives membership.
  const [selectedPaths, setSelectedPaths] = useState<string[]>([]);
  const [renamingPath, setRenamingPath] = useState<string | null>(null);
  const [renameDraft, setRenameDraft] = useState<string>("");

  const activePath = activeProjectPath ?? null;

  // PR-02: shared "+ New project" opener. Prefer the prop override
  // (legacy callers), else the shared dialog context, else send
  // the user to /login.
  const handleNewProject = useCallback(() => {
    if (onNewProject) {
      onNewProject();
      return;
    }
    if (dialog) {
      dialog.open();
      return;
    }
    window.location.href = lp("/login");
  }, [onNewProject, dialog, lp]);

  // Hydrate persisted state + media query listener.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const stored = window.localStorage.getItem(COLLAPSE_KEY);
    if (stored === "true") {
      setCollapsed(true);
    }
    const storedSearch = window.localStorage.getItem(SEARCH_KEY);
    if (storedSearch) {
      setSearch(storedSearch);
    }
    const mql = window.matchMedia(MOBILE_QUERY);
    const onChange = (e: MediaQueryListEvent) => {
      setMobile(e.matches);
      if (e.matches) setCollapsed(true);
    };
    setMobile(mql.matches);
    if (mql.matches) setCollapsed(true);
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, []);

  // Persist collapse + search.
  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(COLLAPSE_KEY, collapsed ? "true" : "false");
  }, [collapsed]);
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (search) {
      window.localStorage.setItem(SEARCH_KEY, search);
    } else {
      window.localStorage.removeItem(SEARCH_KEY);
    }
  }, [search]);

  // Auto-expand the active workspace so the user sees their session.
  useEffect(() => {
    if (!activePath) return;
    setExpanded((cur) => (cur[activePath] ? cur : { ...cur, [activePath]: true }));
  }, [activePath]);

  // Fetch live session list grouped by cwd. Cheap — registry
  // already gives us the list; we just need titles + updated_at.
  useEffect(() => {
    if (collapsed) return;
    let cancelled = false;
    async function load() {
      try {
        const res = await api.get<{
          groups: { omp_cwd: string; sessions: ActiveSession[] }[];
        }>("/api/v1/sessions");
        if (cancelled) return;
        const map: Record<string, ActiveSession[]> = {};
        for (const g of res.groups ?? []) {
          map[g.omp_cwd] = g.sessions;
        }
        setSessions(map);
      } catch {
        // ignore — registry poll will retry
      }
    }
    load();
    const id = setInterval(load, 5000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [collapsed]);

  // PR-07: bulk-select helpers.
  const visiblePaths = useMemo(
    () => projects.filter((p) => !p.hidden).map((p) => p.path),
    [projects],
  );
  const toggleSelected = (path: string) => {
    setSelectedPaths((cur) =>
      cur.includes(path) ? cur.filter((p) => p !== path) : [...cur, path],
    );
  };
  const clearSelected = useCallback(() => setSelectedPaths([]), []);
  const toggleSelectAll = useCallback(() => {
    setSelectedPaths((cur) =>
      cur.length === visiblePaths.length ? [] : visiblePaths,
    );
  }, [visiblePaths]);

  // Workspace list filtered by search (case-insensitive substring).
  const filteredProjects = useMemo(() => {
    const q = search.trim();
    const visible = projects.filter((p) => !p.hidden);
    if (!q) return visible;
    return visible.filter(
      (p) =>
        matchesSearch(p.name, q) ||
        matchesSearch(p.path, q),
    );
  }, [projects, search]);

  const persistedSearch = search.trim();
  const filterActive = persistedSearch.length > 0;

  // Bulk-action toggle: when 2+ projects are selected, expand everyone
  // so the bulk-action bar's "select all" matches the visible tree.
  const toggleWorkspace = (path: string) => {
    setExpanded((cur) => ({ ...cur, [path]: !cur[path] }));
  };

  const startSession = async (cwd: string, projectPath: string | null) => {
    try {
      const res = await api.post<{ id: string }>("/api/v1/sessions", {
        json: projectPath
          ? { omp_cwd: cwd, project_path: projectPath }
          : { omp_cwd: cwd },
      });
      if (res?.id) window.location.href = lp(`/agent/${res.id}`);
    } catch (e) {
      toast.error(e);
    }
  };

  const deleteSession = async (id: string) => {
    try {
      await api.delete(`/api/v1/sessions/${id}`);
    } catch (e) {
      toast.error(e);
    }
  };

  const onCopyPath = async (path: string) => {
    try {
      await navigator.clipboard.writeText(path);
      toast.success(t("ssh.copied"));
    } catch {
      toast.error(t("ssh.error", { message: "copy failed" }));
    }
  };

  const onHideProject = async (path: string) => {
    try {
      await hide(path, true);
    } catch (e) {
      toast.error(e);
    }
  };

  const onRenameProject = (path: string, currentName: string) => {
    setRenamingPath(path);
    setRenameDraft(currentName);
  };

  const commitRename = async () => {
    if (!renamingPath) return;
    const trimmed = renameDraft.trim();
    if (!trimmed) {
      setRenamingPath(null);
      return;
    }
    try {
      await patch(renamingPath, trimmed);
    } catch (e) {
      toast.error(e);
    }
    setRenamingPath(null);
    setRenameDraft("");
  };

  const onRenameKey = (e: ReactKeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      void commitRename();
    } else if (e.key === "Escape") {
      e.preventDefault();
      setRenamingPath(null);
      setRenameDraft("");
    }
  };

  const onStartRenameBlank = (path: string) => {
    const p = projects.find((p) => p.path === path);
    onRenameProject(path, p?.name ?? basename(path));
  };

  const collapseAll = () => setExpanded({});
  const expandAll = () => {
    const next: Record<string, boolean> = {};
    for (const p of filteredProjects) next[p.path] = true;
    setExpanded(next);
  };

  const persistTab = (tab: string) => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(TAB_KEY, tab);
    }
  };

  // --- Collapsed rail --------------------------------------------------

  if (collapsed) {
    return (
      <aside
        className="w-14 shrink-0 border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col items-center py-2 gap-1 transition-[width] duration-200"
        aria-label={t("sidebar.title")}
      >
        <button
          type="button"
          onClick={() => setCollapsed(false)}
          className="rh-button-ghost p-2 rounded-full"
          aria-label={t("sidebar.expand")}
          title={t("sidebar.expand")}
        >
          <PanelLeftOpen className="w-4 h-4" aria-hidden="true" />
        </button>
        <button
          type="button"
          onClick={handleNewProject}
          className="rh-button-primary p-2 rounded-full"
          aria-label={t("sidebar.newSession")}
          title={t("sidebar.newSession")}
        >
          <Plus className="w-4 h-4" aria-hidden="true" />
        </button>
        <div className="flex-1" />
        <BulkActionBar
          selected={selectedPaths}
          allPaths={visiblePaths}
          onClear={clearSelected}
          onToggleSelectAll={toggleSelectAll}
        />
      </aside>
    );
  }

  // --- Expanded tree ---------------------------------------------------

  return (
    <aside
      className="w-64 shrink-0 border-r border-[var(--color-border)] bg-[var(--color-bg-elevated)] flex flex-col h-full transition-[width] duration-200"
      aria-label={t("sidebar.title")}
    >
      {/* Header: brand + collapse */}
      <header className="h-12 px-3 border-b border-[var(--color-border)] flex items-center justify-between gap-2">
        <span className="text-xs font-semibold uppercase tracking-wide text-[var(--color-fg-muted)]">
          {t("app.name")}
        </span>
        <button
          type="button"
          onClick={() => setCollapsed(true)}
          className="rh-button-ghost p-1.5 rounded-full"
          aria-label={t("sidebar.collapse")}
          title={t("sidebar.collapse")}
        >
          <PanelLeftClose className="w-4 h-4" aria-hidden="true" />
        </button>
      </header>

      {/* New session CTA */}
      <div className="px-2 pt-2">
        <button
          type="button"
          onClick={async () => {
            const cwd = activePath ?? visiblePaths[0] ?? null;
            if (!cwd) {
              handleNewProject();
              return;
            }
            await startSession(cwd, cwd);
          }}
          disabled={loading && visiblePaths.length === 0}
          className="w-full flex items-center justify-center gap-2 rounded-full bg-[var(--color-primary)] text-[var(--color-primary-fg)] py-2 text-sm font-medium hover:bg-[var(--color-primary-hover)] disabled:opacity-50"
        >
          <Sparkles className="w-4 h-4" aria-hidden="true" />
          <span>{t("sidebar.newSession")}</span>
        </button>
      </div>

      {/* Search */}
      <div className="px-2 pt-2">
        <label className="relative block">
          <span className="sr-only">{t("sidebar.search")}</span>
          <Search
            className="w-4 h-4 absolute left-2 top-1/2 -translate-y-1/2 text-[var(--color-fg-subtle)] pointer-events-none"
            aria-hidden="true"
          />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("sidebar.search")}
            className="w-full bg-[var(--color-bg-card)] border border-[var(--color-border)] rounded-full pl-8 pr-7 py-1.5 text-sm placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:border-[var(--color-primary)]"
          />
          {search && (
            <button
              type="button"
              onClick={() => setSearch("")}
              className="absolute right-1.5 top-1/2 -translate-y-1/2 p-1 rounded-full text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]"
              aria-label={t("sidebar.search")}
            >
              <X className="w-3.5 h-3.5" aria-hidden="true" />
            </button>
          )}
        </label>
      </div>

      {/* Workspace section header */}
      <div className="px-3 pt-3 pb-1 flex items-center justify-between">
        <span className="text-xs font-medium uppercase tracking-wide text-[var(--color-fg-muted)]">
          {t("sidebar.workspaces")}
        </span>
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={mobile ? collapseAll : expandAll}
            className="rh-button-ghost p-1 rounded-full text-[var(--color-fg-subtle)]"
            aria-label={t("sidebar.collapseAll")}
            title={t("sidebar.collapseAll")}
          >
            <ChevronRight className="w-3.5 h-3.5 rotate-90" aria-hidden="true" />
          </button>
          <button
            type="button"
            onClick={expandAll}
            className="rh-button-ghost p-1 rounded-full text-[var(--color-fg-subtle)]"
            aria-label={t("sidebar.expandAll")}
            title={t("sidebar.expandAll")}
          >
            <ChevronDown className="w-3.5 h-3.5" aria-hidden="true" />
          </button>
        </div>
      </div>

      {/* Tree */}
      <div className="flex-1 overflow-y-auto px-1 pb-2">
        {loading && projects.length === 0 ? (
          <p className="text-xs text-[var(--color-fg-subtle)] px-3 py-2">
            {t("sidebar.loading")}
          </p>
        ) : filteredProjects.length === 0 ? (
          <div className="px-3 py-2 text-xs text-[var(--color-fg-subtle)]">
            {filterActive
              ? t("sidebar.empty.workspaces")
              : t("sidebar.empty.title")}
            {!filterActive && (
              <div className="mt-2">
                <button
                  type="button"
                  onClick={handleNewProject}
                  className="rh-button-primary text-xs px-3 py-1"
                >
                  {t("sidebar.empty.cta")}
                </button>
              </div>
            )}
          </div>
        ) : (
          <ul role="tree" className="flex flex-col">
            {filteredProjects.map((p) => {
              const ws = sessions[p.path] ?? [];
              const isExpanded = !!expanded[p.path];
              const isActive = activePath === p.path;
              const isSelected = selectedPaths.includes(p.path);
              return (
                <WorkspaceRow
                  key={p.path}
                  project={p}
                  isExpanded={isExpanded}
                  isActive={isActive}
                  isSelected={isSelected}
                  isRenaming={renamingPath === p.path}
                  renameDraft={renamingPath === p.path ? renameDraft : ""}
                  onToggle={() => toggleWorkspace(p.path)}
                  onSelect={() => {
                    persistTab("files");
                    onSelectProject?.(p.path);
                  }}
                  onToggleSelect={() => toggleSelected(p.path)}
                  onRenameChange={setRenameDraft}
                  onRenameKey={onRenameKey}
                  onRenameCommit={commitRename}
                  onRenameStart={() => onStartRenameBlank(p.path)}
                  onHide={() => void onHideProject(p.path)}
                  onCopyPath={() => void onCopyPath(p.path)}
                  onStartSession={() => void startSession(p.path, p.path)}
                  actionsT={{
                    copyPath: t("sidebar.project.contextMenu.copyPath"),
                    edit: t("sidebar.project.contextMenu.edit"),
                    hide: t("sidebar.project.contextMenu.hide"),
                    createSession: t("sidebar.project.contextMenu.createSession"),
                    openFiles: t("sidebar.project.contextMenu.openFiles"),
                    settings: t("sidebar.settings"),
                  }}
                  sessionsExpansion={
                    <ul role="group" className="flex flex-col gap-0.5 pt-0.5">
                      {ws.length === 0 ? (
                        <li className="pl-9 pr-2 py-0.5">
                          <button
                            type="button"
                            onClick={() => void startSession(p.path, p.path)}
                            className="text-xs text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] flex items-center gap-1"
                          >
                            <Plus className="w-3 h-3" aria-hidden="true" />
                            {t("sidebar.newSession")}
                          </button>
                        </li>
                      ) : (
                        ws.map((s) => (
                          <SessionRow
                            key={s.id}
                            session={s}
                            active={s.id === activeId}
                            timeAgo={relativeTime(s.updated_at ?? s.created_at, t)}
                            onDelete={() => void deleteSession(s.id)}
                            onOpenFiles={() => persistTab("files")}
                            deleteLabel={t("sidebar.session.row.delete")}
                            openFilesLabel={t("sidebar.project.contextMenu.openFiles")}
                          />
                        ))
                      )}
                    </ul>
                  }
                />
              );
            })}
            {orphans.length > 0 && (
              <OrphanGroup
                orphans={orphans}
                activeId={activeId}
                onStart={(cwd) => void startSession(cwd, null)}
              />
            )}
          </ul>
        )}
      </div>

      {/* Settings card pinned to bottom */}
      <footer className="px-2 pt-2 pb-2 border-t border-[var(--color-border)]">
        <a
          href={lp("/settings")}
          className="flex items-center gap-2 px-3 py-2 rounded-md hover:bg-[var(--color-bg-card)] text-sm"
        >
          <Settings className="w-4 h-4 text-[var(--color-fg-muted)]" aria-hidden="true" />
          <span>{t("sidebar.settings")}</span>
        </a>
      </footer>

      <BulkActionBar
        selected={selectedPaths}
        allPaths={visiblePaths}
        onClear={clearSelected}
        onToggleSelectAll={toggleSelectAll}
      />
    </aside>
  );
}

// --- WorkspaceRow -------------------------------------------------------

interface WorkspaceRowProps {
  project: RegistryProject;
  isExpanded: boolean;
  isActive: boolean;
  isSelected: boolean;
  isRenaming: boolean;
  renameDraft: string;
  onToggle: () => void;
  onSelect: () => void;
  onToggleSelect: () => void;
  onRenameStart: () => void;
  onRenameChange: (value: string) => void;
  onRenameKey: (e: ReactKeyboardEvent<HTMLInputElement>) => void;
  onRenameCommit: () => void;
  onHide: () => void;
  onCopyPath: () => void;
  onStartSession: () => void;
  sessionsExpansion: React.ReactNode;
  actionsT: {
    copyPath: string;
    edit: string;
    hide: string;
    createSession: string;
    openFiles: string;
    settings: string;
  };
}

function WorkspaceRow({
  project,
  isExpanded,
  isActive,
  isSelected,
  isRenaming,
  renameDraft,
  onToggle,
  onSelect,
  onToggleSelect,
  onRenameStart,
  onRenameChange,
  onRenameKey,
  onRenameCommit,
  onHide,
  onCopyPath,
  onStartSession,
  sessionsExpansion,
  actionsT,
}: WorkspaceRowProps) {
  const menuRef = useRef<HTMLDivElement>(null);
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    if (!menuOpen) return;
    function onPointerDown(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", onPointerDown);
    return () => document.removeEventListener("mousedown", onPointerDown);
  }, [menuOpen]);

  return (
    <li
      role="treeitem"
      aria-expanded={isExpanded}
      aria-selected={isActive}
      data-active={isActive}
      className="group relative"
    >
      <div
        className={`flex items-center gap-1 rounded-md px-1.5 py-1 text-sm cursor-pointer ${
          isActive
            ? "bg-[var(--color-bg-card)] text-[var(--color-fg)]"
            : "hover:bg-[var(--color-bg-card)] text-[var(--color-fg)]"
        }`}
      >
        <button
          type="button"
          onClick={onToggle}
          aria-label={actionsT.settings}
          className="p-0.5 text-[var(--color-fg-muted)]"
        >
          {isExpanded ? (
            <ChevronDown className="w-3.5 h-3.5" aria-hidden="true" />
          ) : (
            <ChevronRight className="w-3.5 h-3.5" aria-hidden="true" />
          )}
        </button>
        <input
          type="checkbox"
          className="mr-1"
          checked={isSelected}
          onChange={onToggleSelect}
          onClick={(e) => e.stopPropagation()}
          aria-label={`Select ${project.name}`}
        />
        <button
          type="button"
          onClick={() => {
            onSelect();
            if (!isExpanded) onToggle();
          }}
          className="flex items-center gap-1.5 flex-1 min-w-0 text-left"
        >
          {isExpanded ? (
            <FolderOpen className="w-4 h-4 text-[var(--color-fg-muted)] shrink-0" aria-hidden="true" />
          ) : (
            <Folder className="w-4 h-4 text-[var(--color-fg-muted)] shrink-0" aria-hidden="true" />
          )}
          {isRenaming ? (
            <input
              type="text"
              autoFocus
              value={renameDraft}
              onChange={(e) => onRenameChange(e.target.value)}
              onKeyDown={onRenameKey}
              onBlur={onRenameCommit}
              onClick={(e) => e.stopPropagation()}
              className="flex-1 min-w-0 bg-[var(--color-bg-elevated)] border border-[var(--color-border)] rounded px-1 text-xs"
            />
          ) : (
            <span className="truncate" title={project.name}>
              {project.name}
            </span>
          )}
        </button>
        <div
          ref={menuRef}
          className="relative opacity-0 group-hover:opacity-100 focus-within:opacity-100"
        >
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              setMenuOpen((v) => !v);
            }}
            aria-label={`Workspace actions for ${project.name}`}
            className="p-1 rounded text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            <MoreHorizontal className="w-3.5 h-3.5" aria-hidden="true" />
          </button>
          {menuOpen && (
            <div
              role="menu"
              className="absolute right-0 top-full mt-1 z-30 min-w-44 border border-[var(--color-border)] rounded-md bg-[var(--color-bg-elevated)] shadow-lg py-1 text-sm"
            >
              <MenuItem
                label={actionsT.createSession}
                icon={<Plus className="w-3.5 h-3.5" aria-hidden="true" />}
                onClick={() => {
                  setMenuOpen(false);
                  onStartSession();
                }}
              />
              <MenuItem
                label={actionsT.edit}
                icon={<Pencil className="w-3.5 h-3.5" aria-hidden="true" />}
                onClick={() => {
                  setMenuOpen(false);
                  onRenameStart();
                }}
              />
              <MenuItem
                label={actionsT.copyPath}
                icon={<Copy className="w-3.5 h-3.5" aria-hidden="true" />}
                onClick={() => {
                  setMenuOpen(false);
                  onCopyPath();
                }}
              />
              <MenuItem
                label={actionsT.hide}
                icon={<EyeOff className="w-3.5 h-3.5" aria-hidden="true" />}
                onClick={() => {
                  setMenuOpen(false);
                  onHide();
                }}
              />
            </div>
          )}
        </div>
      </div>
      {isExpanded && sessionsExpansion}
    </li>
  );
}

function MenuItem({
  label,
  icon,
  onClick,
  danger = false,
}: {
  label: string;
  icon?: React.ReactNode;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      role="menuitem"
      onClick={onClick}
      className={`w-full text-left px-3 py-1.5 flex items-center gap-2 hover:bg-[var(--color-bg-card)] ${
        danger ? "text-[var(--color-danger)]" : "text-[var(--color-fg)]"
      }`}
    >
      {icon}
      <span>{label}</span>
    </button>
  );
}

// --- SessionRow --------------------------------------------------------

interface SessionRowProps {
  session: ActiveSession;
  active: boolean;
  timeAgo: string;
  onDelete: () => void;
  onOpenFiles: () => void;
  deleteLabel: string;
  openFilesLabel: string;
}

function SessionRow({
  session,
  active,
  timeAgo,
  onDelete,
  onOpenFiles,
  deleteLabel,
  openFilesLabel,
}: SessionRowProps) {
  const lp = useLocalizedPath();
  const menuRef = useRef<HTMLDivElement>(null);
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    if (!menuOpen) return;
    function onPointerDown(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", onPointerDown);
    return () => document.removeEventListener("mousedown", onPointerDown);
  }, [menuOpen]);

  return (
    <li
      role="treeitem"
      aria-selected={active}
      className={`group relative flex items-center gap-1 rounded-md px-1.5 py-1 text-xs ${
        active
          ? "bg-[var(--color-bg-card)]"
          : "hover:bg-[var(--color-bg-card)]"
      }`}
    >
      <span className="pl-3 text-[var(--color-fg-subtle)]" aria-hidden="true">
        •
      </span>
      <a
        href={lp(`/agent/${session.id}`)}
        className="flex-1 min-w-0 truncate"
        title={session.title || session.id}
      >
        {session.title || session.id}
      </a>
      <span className="text-[10px] text-[var(--color-fg-subtle)] shrink-0">
        {timeAgo}
      </span>
      <div
        ref={menuRef}
        className="relative opacity-0 group-hover:opacity-100 focus-within:opacity-100"
      >
        <button
          type="button"
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            setMenuOpen((v) => !v);
          }}
          aria-label={`Session actions for ${session.title || session.id}`}
          className="p-1 rounded text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
        >
          <MoreHorizontal className="w-3.5 h-3.5" aria-hidden="true" />
        </button>
        {menuOpen && (
          <div
            role="menu"
            className="absolute right-0 top-full mt-1 z-30 min-w-40 border border-[var(--color-border)] rounded-md bg-[var(--color-bg-elevated)] shadow-lg py-1 text-sm"
          >
            <MenuItem
              label={openFilesLabel}
              icon={<FolderOpen className="w-3.5 h-3.5" aria-hidden="true" />}
              onClick={() => {
                setMenuOpen(false);
                onOpenFiles();
              }}
            />
            <MenuItem
              label={deleteLabel}
              icon={<Trash2 className="w-3.5 h-3.5" aria-hidden="true" />}
              onClick={() => {
                setMenuOpen(false);
                onDelete();
              }}
              danger
            />
          </div>
        )}
      </div>
    </li>
  );
}

// --- OrphanGroup -------------------------------------------------------

interface OrphanGroupProps {
  orphans: OrphanSession[];
  activeId?: string;
  onStart: (cwd: string) => void;
}

function OrphanGroup({ orphans, activeId, onStart }: OrphanGroupProps) {
  const t = useT();
  const lp = useLocalizedPath();
  const [expanded, setExpanded] = useState(true);
  return (
    <li className="mt-2 rounded-md bg-[var(--color-bg-card)] border border-dashed border-[var(--color-border)]">
      <button
        type="button"
        onClick={() => setExpanded((e) => !e)}
        className="w-full flex items-center justify-between px-2 py-1 text-xs text-[var(--color-fg-muted)]"
      >
        <span>{t("sidebar.others")}</span>
        <span className="flex items-center gap-1">
          {orphans.length}
          {expanded ? (
            <ChevronDown className="w-3 h-3" aria-hidden="true" />
          ) : (
            <ChevronRight className="w-3 h-3" aria-hidden="true" />
          )}
        </span>
      </button>
      {expanded && (
        <ul className="flex flex-col gap-0.5 px-1 pb-1">
          {orphans.map((o) => (
            <li
              key={o.id}
              className={`flex items-center gap-1 rounded-md px-1.5 py-1 text-xs ${
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
                {basename(o.omp_cwd) || o.omp_cwd}
              </a>
              <button
                type="button"
                onClick={() => onStart(o.omp_cwd)}
                className="text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)]"
                aria-label={t("sidebar.newSession")}
              >
                <Plus className="w-3 h-3" aria-hidden="true" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}
