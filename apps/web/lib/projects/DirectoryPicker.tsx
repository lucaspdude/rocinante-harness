"use client";

// DirectoryPicker — modal browser for the host filesystem, used
// by CreateProjectDialog's "Folder" / "Empty" / "Clone" tabs.
// Ported from /tmp/rocinante-ref/components/DirectoryPicker.tsx
// with two adaptations:
//
//   - The reference used Base UI + CSS variables for styling;
//     this repo uses the rh-card / rh-input / rh-button-primary
//     Tailwind component classes from apps/web/app/globals.css.
//   - The reference used useModalDialog for focus-trap + ESC;
//     this repo doesn't have that hook yet, so we omit focus
//     management (acceptable for the picker — backdrop click
//     and Cancel button cover the close paths).
//
// Endpoint: GET /api/v1/cwd/browse?path=...

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { api } from "../api/client";
import { useMe } from "../me/useMe";
import { useT } from "../i18n";
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
  const { me } = useMe();
  const home = me?.home ?? "/root";
  const registered = new Set(registeredPaths ?? []);

  const [currentPath, setCurrentPath] = useState<string>(home);
  const [parentPath, setParentPath] = useState<string | null>(null);
  const [pathInput, setPathInput] = useState<string>(home);
  const [entries, setEntries] = useState<DirectoryEntry[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
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
        const res = await api.get<BrowseResponse>(
          `/api/v1/cwd/browse?path=${encodeURIComponent(expanded)}`,
        );
        setCurrentPath(res.path);
        setParentPath(res.parent ?? null);
        setPathInput(res.path);
        setEntries(res.entries ?? []);
      } catch (e: unknown) {
        const err = e as {
          body?: { message?: string; code?: string };
          message?: string;
        };
        setLoadError(err.body?.message ?? err.message ?? "failed");
      } finally {
        setLoading(false);
      }
    },
    [home],
  );

  // Initial mount: navigate to the user's home dir once we have it.
  useEffect(() => {
    if (me) void navigateTo(me.home);
    // We intentionally don't add navigateTo here — it's a stable
    // callback over home, and re-running on each navigateTo
    // identity change would loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [me]);

  const handlePathSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const candidate = pathInput.trim();
    if (candidate) void navigateTo(candidate);
  };

  const handleEntryClick = (entry: DirectoryEntry) => {
    if (!entry.is_dir) return;
    void navigateTo(joinPath(currentPath, entry.name));
  };

  const hasUncommittedPath = pathInput.trim() !== currentPath;
  const canSelect = Boolean(currentPath) && !hasUncommittedPath && !busy;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="directory-picker-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
    >
      <div className="rh-card w-full max-w-xl max-h-[90vh] flex flex-col overflow-hidden">
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
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
            disabled={busy}
          >
            ×
          </button>
        </header>

        <form
          onSubmit={handlePathSubmit}
          className="flex items-center gap-2 mb-3"
        >
          <button
            type="button"
            onClick={() => parentPath && void navigateTo(parentPath)}
            disabled={loading || !parentPath}
            title={t("projects.folderPicker.upHint")}
            aria-label={t("projects.folderPicker.up")}
            className="rh-button-ghost h-9 w-9 p-0 flex items-center justify-center"
          >
            <span aria-hidden="true">↑</span>
          </button>
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
            placeholder={t("projects.folderPicker.title")}
            autoFocus
            spellCheck={false}
            className="rh-input font-mono text-sm flex-1"
          />
          <button
            type="submit"
            disabled={loading || !pathInput.trim()}
            className="rh-button-ghost text-sm"
          >
            {t("projects.folderPicker.open")}
          </button>
        </form>

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
          ) : entries.length > 0 ? (
            <ul role="list" className="p-1">
              {entries.map((entry) => {
                const fullPath = joinPath(currentPath, entry.name);
                const inRegistry = registered.has(fullPath);
                return (
                  <li key={entry.name}>
                    <button
                      type="button"
                      onClick={() => handleEntryClick(entry)}
                      disabled={!entry.is_dir}
                      title={fullPath}
                      className="w-full text-left px-2 py-1.5 rounded font-mono text-xs hover:bg-[var(--color-bg-elevated)] disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                    >
                      <span aria-hidden="true">
                        {entry.is_dir ? "�" : "📄"}
                      </span>
                      <span className="truncate flex-1">{entry.name}</span>
                      {inRegistry && (
                        <span
                          className="text-[10px] uppercase tracking-wide px-1.5 py-0.5 rounded bg-[var(--color-primary)] text-[var(--color-primary-fg)]"
                          aria-label={t("projects.folderPicker.inRegistry")}
                        >
                          {t("projects.folderPicker.inRegistry")}
                        </span>
                      )}
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
