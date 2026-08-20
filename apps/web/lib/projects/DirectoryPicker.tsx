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
//
// PR-03 changes:
//   - D1: bootstrap navigateTo uses "/" when useMe returns the
//     fallback (no token); the picker no longer waits forever
//     for the user-home effect to run.
//   - D2: load errors surface as toasts; no inline <p role="alert">.
//   - D3: pathInput + handlePathSubmit are always enabled — the
//     user can still type /root/projects/foo and submit even when
//     browse fails.
//   - D5: when the last browse failed with 401, a small CTA row
//     appears with "Sign in" / "Retry" so the user has a way out.

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
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
  const [loadError, setLoadError] = useState<string | null>(null);
  // PR-03 D5 — track whether the last failed browse was a 401
  // so we can show the "Sign in / Retry" CTA row.
  const [authFailed, setAuthFailed] = useState<boolean>(false);
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
        // The browse endpoint is allow-list gated server-side ($HOME +
        // registered projects), so it is safe without a bearer token —
        // and the picker has to work before the user signs in.
        const res = await api.get<BrowseResponse>(
          `/api/v1/cwd/browse?path=${encodeURIComponent(expanded)}`,
          { unauthenticated: true },
        );
        setCurrentPath(res.path);
        setParentPath(res.parent ?? null);
        setPathInput(res.path);
        setEntries(res.entries ?? []);
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
                        {entry.is_dir ? "📁" : "📄"}
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