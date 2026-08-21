"use client";

// DirectoryPicker — macOS-Finder-style modal browser for the host
// filesystem, used by CreateProjectDialog's "Folder" / "Clone" /
// "Empty" tabs.
//
// PR-03 rewrite: Finder-style breadcrumb, search filter, chevron
// list rows, keyboard navigation (ArrowDown/Up/Enter/Esc). The
// 401 signed-out CTA row, toast-on-error reporting, and bearer-
// token transport from the previous implementation carry over
// verbatim.
//
// Endpoint: GET /api/v1/cwd/browse?path=...
//
// History: lifted from the candystore picker (PR-02 wire). The
// original used Base UI + CSS variables; this repo uses the
// rh-card / rh-input / rh-button-* Tailwind component classes from
// apps/web/app/globals.css. The original had no focus-trap (the
// repo doesn't ship useModalDialog yet); the new picker focuses
// the list on open so arrow keys work immediately, and Esc
// cancels from anywhere inside the dialog.

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
} from "react";
import {
  ChevronUp,
  ChevronRight,
  Folder,
  Home as HomeIcon,
  Search as SearchIcon,
  X,
} from "lucide-react";
import { api, ApiClientError } from "../api/client";
import { useMe } from "../me/useMe";
import { useT, useLocalizedPath } from "../i18n";
import { useToast } from "../toast";
import type { Project } from "./useProjects";

interface DirectoryEntry {
  name: string;
  is_dir: boolean;
  size: number;
  mode: string;
  mod_time: string;
}

interface BrowseResponse {
  path: string;
  parent?: string;
  entries: DirectoryEntry[];
}

interface Props {
  /** When false the picker renders nothing. Defaults to true. */
  open?: boolean;
  onCancel: () => void;
  onSelect: (path: string) => void;
  /** Optional set of absolute paths already in the registry — the picker renders an "in registry" hint next to them. */
  registeredPaths?: string[];
  busy?: boolean;
  /** Optional parent-dialog error to surface (e.g. "expand failed"). */
  error?: string | null;
}

function stripTrailingSlash(p: string): string {
  if (p.length > 1 && p.endsWith("/")) return p.replace(/\/+$/, "");
  return p;
}

function joinPath(parent: string, child: string): string {
  if (!parent || parent === "/") return `/${child}`;
  return `${parent.replace(/\/+$/, "")}/${child}`;
}

interface Crumb {
  /** text shown in the breadcrumb button */
  label: string;
  /** absolute path the button navigates to */
  full: string;
  /** whether this is the "/" root crumb (rendered with icon + "Computer") */
  isRoot: boolean;
}

/** Split `currentPath` into clickable breadcrumb segments. */
function splitSegments(path: string): Crumb[] {
  const cleaned = stripTrailingSlash(path);
  if (cleaned === "" || cleaned === "/") {
    return [{ label: "Computer", full: "/", isRoot: true }];
  }
  const parts = cleaned.split("/").filter(Boolean);
  const out: Crumb[] = [];
  let acc = "";
  for (const part of parts) {
    acc = `${acc}/${part}`;
    out.push({ label: part, full: acc, isRoot: false });
  }
  return out;
}

export function DirectoryPicker(props: Props) {
  if (props.open === false) return null;
  return <DirectoryPickerInner {...props} />;
}

function DirectoryPickerInner({
  onCancel,
  onSelect,
  registeredPaths,
  busy = false,
  error,
}: Props) {
  const t = useT();
  const toast = useToast();
  const lp = useLocalizedPath();
  const { me } = useMe();
  const home = me?.home ?? "/";
  const registered = new Set(registeredPaths ?? []);

  const [currentPath, setCurrentPath] = useState<string>(home);
  const [parentPath, setParentPath] = useState<string | null>(null);
  const [pathInput, setPathInput] = useState<string>(home);
  const [entries, setEntries] = useState<DirectoryEntry[]>([]);
  const [search, setSearch] = useState<string>("");
  const [selectedIndex, setSelectedIndex] = useState<number>(0);
  const [loadError, setLoadError] = useState<string | null>(null);
  // PR-03 D5 — track whether the last failed browse was a 401
  // so we can show the "Sign in / Retry" CTA row.
  const [authFailed, setAuthFailed] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(true);
  const listRef = useRef<HTMLUListElement>(null);
  const lastLoadErrorRef = useRef<string | null>(null);
  const lastPropErrorRef = useRef<string | null>(null);

  useEffect(() => {
    if (loadError && loadError !== lastLoadErrorRef.current) {
      lastLoadErrorRef.current = loadError;
      toast.error(loadError);
    }
  }, [loadError, toast]);

  useEffect(() => {
    if (error && error !== lastPropErrorRef.current) {
      lastPropErrorRef.current = error;
      toast.error(error);
    }
  }, [error, toast]);

  const navigateTo = useCallback(
    async (directory?: string) => {
      setLoading(true);
      setLoadError(null);
      setAuthFailed(false);
      try {
        const target = stripTrailingSlash(directory ?? home);
        // Expand "~" client-side so the api call sends an absolute
        // path. The api also expands server-side, but this keeps
        // the displayed breadcrumb consistent.
        const expanded =
          target === "~"
            ? home
            : target.startsWith("~/")
            ? home + target.slice(1)
            : target;
        // /cwd/browse sits behind AuthMW, so the request must carry the
        // bearer token when one exists. api.get omits the header when
        // signed out, which yields the 401 the "Sign in" CTA below
        // handles.
        const res = await api.get<BrowseResponse>(
          `/api/v1/cwd/browse?path=${encodeURIComponent(expanded)}`,
        );
        setCurrentPath(res.path);
        setParentPath(res.parent ?? null);
        setPathInput(res.path);
        setEntries(res.entries ?? []);
        setSearch("");
        setSelectedIndex(0);
      } catch (e: unknown) {
        const err = e as {
          status?: number;
          body?: { message?: string; code?: string };
          message?: string;
        };
        const isApiError = e instanceof ApiClientError;
        const status = isApiError ? (e as ApiClientError).status : err.status;
        const msg =
          err.body?.message ?? err.message ?? t("projects.folderPicker.empty");
        setLoadError(msg);
        if (status === 401) {
          // PR-03 D5: surface the CTA row.
          setAuthFailed(true);
          toast.error(
            t("projects.folderPicker.signedOut"),
            t("projects.folderPicker.signedOutHint"),
          );
          // PR-03 D1: if the user tried "/" itself, surface a
          // specific message instead of the generic 401.
          if ((directory ?? home) === "/") {
            toast.error(t("projects.folderPicker.cannotListRoot"));
          }
        } else {
          toast.error(msg);
        }
      } finally {
        setLoading(false);
      }
    },
    [home, t, toast],
  );

  // Initial mount: navigate to the user's home dir (or "/" when
  // the fallback is in effect). PR-03 D1: never guard on `me` —
  // wait for it would freeze the picker when no token is present.
  useEffect(() => {
    void navigateTo(me?.home ?? "/");
    // We intentionally don't add navigateTo here — it's a stable
    // callback over home, and re-running on each navigateTo
    // identity change would loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handlePathSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const candidate = pathInput.trim();
    if (candidate) void navigateTo(candidate);
  };

  const handleEntryClick = (entry: DirectoryEntry) => {
    if (!entry.is_dir) return;
    void navigateTo(joinPath(currentPath, entry.name));
  };

  const handleRetry = () => {
    void navigateTo(currentPath);
  };

  // Filtered entries — case-insensitive substring match on name.
  const visibleEntries = search
    ? entries.filter((e) =>
        e.name.toLowerCase().includes(search.toLowerCase()),
      )
    : entries;

  // Clamp selectedIndex when the filter list shrinks.
  useEffect(() => {
    if (visibleEntries.length === 0) {
      if (selectedIndex !== 0) setSelectedIndex(0);
      return;
    }
    if (selectedIndex >= visibleEntries.length) {
      setSelectedIndex(visibleEntries.length - 1);
    }
  }, [visibleEntries.length, selectedIndex]);

  // Re-focus the list whenever navigation completes so arrow keys
  // work immediately after the user clicks a folder.
  useEffect(() => {
    if (!loading && listRef.current) {
      listRef.current.focus();
    }
  }, [loading, currentPath]);

  const handleListKeyDown = (event: KeyboardEvent<HTMLUListElement>) => {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      if (visibleEntries.length === 0) return;
      setSelectedIndex((i) => (i + 1) % visibleEntries.length);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      if (visibleEntries.length === 0) return;
      setSelectedIndex(
        (i) => (i - 1 + visibleEntries.length) % visibleEntries.length,
      );
    } else if (event.key === "Enter") {
      event.preventDefault();
      const entry = visibleEntries[selectedIndex];
      if (entry?.is_dir) {
        void navigateTo(joinPath(currentPath, entry.name));
      }
    }
  };

  const handleDialogKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    // Esc cancels regardless of focus. Other keys fall through to
    // the inner list/input handlers so typing in the search box
    // works normally.
    if (event.key === "Escape") {
      event.preventDefault();
      onCancel();
    }
  };

  const hasUncommittedPath = pathInput.trim() !== currentPath;
  const canSelect = Boolean(currentPath) && !hasUncommittedPath && !busy;

  const segments = splitSegments(currentPath);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="directory-picker-title"
      onKeyDown={handleDialogKeyDown}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
    >
      <div className="rh-card w-full max-w-2xl max-h-[90vh] flex flex-col overflow-hidden">
        <header className="flex items-center justify-between mb-3">
          <h2
            id="directory-picker-title"
            className="text-base font-medium"
          >
            {t("projects.folderPicker.title")}
          </h2>
          <button
            type="button"
            onClick={onCancel}
            aria-label={t("common.close")}
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] disabled:opacity-50"
            disabled={busy}
          >
            <X size={18} aria-hidden="true" />
          </button>
        </header>

        {authFailed && (
          <div
            role="status"
            className="mb-3 flex flex-wrap items-center justify-between gap-2 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-3 py-2 text-xs"
          >
            <span className="text-[var(--color-fg-muted)]">
              {t("projects.folderPicker.cta.helpText")}
            </span>
            <span className="flex items-center gap-3">
              <a
                href={lp("/login")}
                className="font-medium underline text-[var(--color-primary)] hover:opacity-80"
              >
                {t("projects.folderPicker.cta.signIn")}
              </a>
              <button
                type="button"
                onClick={handleRetry}
                className="rh-button-ghost h-7 px-2 text-xs"
              >
                {t("projects.folderPicker.cta.retry")}
              </button>
            </span>
          </div>
        )}

        {/* Breadcrumb row: Up button + clickable path segments. */}
        <div className="flex items-center gap-2 mb-2">
          <button
            type="button"
            onClick={() => parentPath && void navigateTo(parentPath)}
            disabled={loading || !parentPath}
            title={t("projects.folderPicker.up")}
            aria-label={t("projects.folderPicker.up")}
            className="rh-button-ghost h-9 w-9 p-0 flex items-center justify-center flex-shrink-0"
          >
            <ChevronUp size={18} aria-hidden="true" />
          </button>
          <nav
            aria-label="breadcrumb"
            className="flex items-center flex-1 min-w-0 overflow-x-auto text-sm h-9 px-2 rounded border border-[var(--color-border)] bg-[var(--color-bg)]"
          >
            {segments.map((seg, i) => (
              <span
                key={seg.full}
                className="flex items-center flex-shrink-0"
              >
                {i > 0 && (
                  <ChevronRight
                    size={14}
                    aria-hidden="true"
                    className="mx-0.5 text-[var(--color-fg-muted)] flex-shrink-0"
                  />
                )}
                <button
                  type="button"
                  onClick={() => void navigateTo(seg.full)}
                  disabled={loading}
                  className={`px-1 py-0.5 rounded hover:bg-[var(--color-bg-elevated)] whitespace-nowrap flex items-center gap-1 ${
                    i === segments.length - 1
                      ? "font-medium text-[var(--color-fg)]"
                      : "text-[var(--color-fg-muted)]"
                  }`}
                  title={seg.full}
                >
                  {seg.isRoot ? (
                    <>
                      <HomeIcon size={14} aria-hidden="true" />
                      {t("projects.folderPicker.breadcrumbHome")}
                    </>
                  ) : (
                    seg.label
                  )}
                </button>
              </span>
            ))}
          </nav>
        </div>

        {/* Path input — direct entry, Enter to navigate. */}
        <form onSubmit={handlePathSubmit} className="mb-2">
          <label htmlFor="directory-path" className="sr-only">
            {t("projects.folderPicker.title")}
          </label>
          <input
            id="directory-path"
            type="text"
            value={pathInput}
            onChange={(e) => {
              setPathInput(e.target.value);
              setLoadError(null);
            }}
            placeholder="/path/to/folder"
            spellCheck={false}
            className="rh-input font-mono text-xs w-full"
          />
        </form>

        {/* Search filter — narrows the visible folder list. */}
        <div className="mb-2 relative">
          <label htmlFor="directory-search" className="sr-only">
            {t("projects.folderPicker.search")}
          </label>
          <SearchIcon
            size={14}
            aria-hidden="true"
            className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--color-fg-muted)] pointer-events-none"
          />
          <input
            id="directory-search"
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("projects.folderPicker.search")}
            spellCheck={false}
            className="rh-input text-sm w-full pl-8"
          />
        </div>

        <div
          className="flex-1 min-h-0 overflow-auto border border-[var(--color-border)] rounded"
          aria-busy={loading}
        >
          {loading ? (
            <ul className="p-2 space-y-1" aria-label="loading">
              {Array.from({ length: 5 }).map((_, i) => (
                <li
                  key={i}
                  className="h-7 rounded bg-[var(--color-bg-elevated)] animate-pulse"
                  style={{ width: `${55 + ((i * 37) % 40)}%` }}
                />
              ))}
            </ul>
          ) : visibleEntries.length > 0 ? (
            <ul
              ref={listRef}
              role="list"
              tabIndex={0}
              onKeyDown={handleListKeyDown}
              className="p-1 outline-none"
            >
              {visibleEntries.map((entry, i) => {
                const fullPath = joinPath(currentPath, entry.name);
                const inRegistry = registered.has(fullPath);
                const isSelected = i === selectedIndex;
                return (
                  <li key={entry.name}>
                    <button
                      type="button"
                      onClick={() => handleEntryClick(entry)}
                      onMouseEnter={() => setSelectedIndex(i)}
                      disabled={!entry.is_dir}
                      title={fullPath}
                      data-selected={isSelected}
                      className={`w-full text-left px-2 py-1.5 rounded text-sm flex items-center gap-2 ${
                        isSelected
                          ? "bg-[var(--color-primary)] text-[var(--color-primary-fg)]"
                          : "hover:bg-[var(--color-bg-elevated)]"
                      } disabled:opacity-50 disabled:cursor-not-allowed`}
                    >
                      <Folder
                        size={16}
                        aria-hidden="true"
                        className="flex-shrink-0"
                      />
                      <span className="truncate flex-1">{entry.name}</span>
                      {inRegistry && (
                        <span
                          className="text-[10px] uppercase tracking-wide px-1.5 py-0.5 rounded bg-[var(--color-primary)] text-[var(--color-primary-fg)] flex-shrink-0"
                          aria-label={t("projects.folderPicker.inRegistry")}
                        >
                          {t("projects.folderPicker.inRegistry")}
                        </span>
                      )}
                      <ChevronRight
                        size={16}
                        aria-hidden="true"
                        className="flex-shrink-0 opacity-60"
                      />
                    </button>
                  </li>
                );
              })}
            </ul>
          ) : (
            <p className="p-3 text-sm text-[var(--color-fg-muted)]">
              {t("projects.folderPicker.empty")}
            </p>
          )}
        </div>

        <footer className="mt-3 pt-3 flex justify-end gap-2">
          <button
            type="button"
            onClick={onCancel}
            disabled={busy}
            className="rh-button-ghost text-sm"
          >
            {t("projects.folderPicker.cancel")}
          </button>
          <button
            type="button"
            onClick={() => onSelect(currentPath)}
            disabled={!canSelect}
            className="rh-button-primary text-sm"
          >
            {t("projects.folderPicker.select")}
          </button>
        </footer>
      </div>
    </div>
  );
}

// Re-exported for callers that want to highlight which entries
// already correspond to a registered project.
export function getRegisteredPaths(projects: Project[]): string[] {
  return projects.map((p) => p.path);
}
