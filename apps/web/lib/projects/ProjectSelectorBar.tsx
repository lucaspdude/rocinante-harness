"use client";

// PR-02: chat-first project selector bar.
//
// Sits at the top of the /agent page. Shows the currently-selected
// project name (or "No project selected"). Click opens a dropdown
// with the registered projects plus "+ New project". Selecting a
// project writes rh:selected-project-path to localStorage. The
// "+ New project" item opens the shared CreateProjectDialog via
// the CreateProjectDialogProvider context.
//
// Quick win: Cmd+K (or Ctrl+K) toggles the dropdown so the user
// can switch projects from the keyboard.

import { useEffect, useRef, useState } from "react";
import { useT, useLocalizedPath } from "../i18n";
import { ApiClientError } from "../api/client";
import { useCreateProjectDialog } from "./CreateProjectDialogProvider";
import type { Project } from "./useProjects";

interface Props {
  projects: Project[];
  loading: boolean;
  error: string | null;
  selectedPath: string | null;
  onSelect: (path: string | null) => void;
}

function isAuthMissing(err: string | null): boolean {
  if (!err) return false;
  // useProjects stores err.body?.message or err.message as a string;
  // we can't see the structured ApiClientError body there. To detect
  // auth we instead probe the text — the api returns the i18n-keyed
  // message which currently is empty for these codes. Easier: the
  // page can also pass an empty projects list when 401 happens.
  // We treat the empty-state + null projects as "defer to parent's
  // sign-in CTA" and only show the CTA when the api explicitly tells
  // us we have no token. For PR-02 we rely on the parent's auth-
  // missing handling: see AgentHomePage.
  return false;
}

export function ProjectSelectorBar({
  projects,
  loading,
  error: _error,
  selectedPath,
  onSelect,
}: Props) {
  const t = useT();
  const lp = useLocalizedPath();
  const dialog = useCreateProjectDialog();
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Click-outside closes the dropdown.
  useEffect(() => {
    if (!open) return;
    function onPointerDown(e: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onPointerDown);
    return () => document.removeEventListener("mousedown", onPointerDown);
  }, [open]);

  // Cmd+K / Ctrl+K toggles the dropdown. Ignored when typing in a field.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        const target = e.target as HTMLElement | null;
        const tag = target?.tagName?.toLowerCase();
        if (
          tag === "input" ||
          tag === "textarea" ||
          target?.isContentEditable
        ) {
          return;
        }
        e.preventDefault();
        setOpen((v) => !v);
      }
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  // On 401 we'd show a sign-in CTA. The string-only `error` from
  // useProjects doesn't carry the structured code, so leave the bar
  // usable; the dropdown may simply show "no projects yet". Hooking
  // this up properly would require restructuring useProjects to
  // surface ApiClientError — that's PR-03 (picker) territory.
  void isAuthMissing;

  const current = selectedPath
    ? projects.find((p) => p.path === selectedPath) ?? null
    : null;
  const label = current ? current.name : t("projectSelector.none");

  return (
    <div
      ref={containerRef}
      className="relative border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)]"
    >
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center justify-between gap-2 px-4 py-2 text-left hover:bg-[var(--color-bg-card)] transition-colors"
        aria-haspopup="listbox"
        aria-expanded={open}
        data-testid="project-selector-trigger"
      >
        <span className="text-xs uppercase tracking-wide text-[var(--color-fg-muted)]">
          {t("projectSelector.title")}
        </span>
        <span className="flex-1 text-sm font-medium truncate ml-2">
          {label}
        </span>
        <span className="text-xs text-[var(--color-fg-muted)]">
          {open ? "▴" : "▾"}
        </span>
      </button>
      {open && (
        <div
          role="listbox"
          className="absolute top-full left-0 right-0 z-40 mt-1 border border-[var(--color-border)] rounded bg-[var(--color-bg-elevated)] shadow-lg max-h-80 overflow-y-auto"
        >
          {loading && projects.length === 0 ? (
            <p className="p-3 text-sm text-[var(--color-fg-muted)]">
              {t("common.loading")}
            </p>
          ) : projects.length === 0 ? (
            <p className="p-3 text-sm text-[var(--color-fg-muted)]">
              {t("projectSelector.emptyHint")}
            </p>
          ) : (
            <ul>
              {projects.map((p) => (
                <li key={p.path}>
                  <button
                    type="button"
                    onClick={() => {
                      onSelect(p.path);
                      setOpen(false);
                    }}
                    data-active={selectedPath === p.path}
                    className={`w-full text-left px-3 py-2 hover:bg-[var(--color-bg-card)] ${
                      selectedPath === p.path
                        ? "bg-[var(--color-bg-card)]"
                        : ""
                    }`}
                    role="option"
                    aria-selected={selectedPath === p.path}
                  >
                    <div className="text-sm font-medium truncate">
                      {p.name}
                    </div>
                    <div className="text-xs text-[var(--color-fg-muted)] font-mono truncate">
                      {p.path}
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          )}
          <div className="border-t border-[var(--color-border)]">
            <button
              type="button"
              onClick={() => {
                setOpen(false);
                dialog?.open();
              }}
              className="w-full text-left px-3 py-2 text-sm hover:bg-[var(--color-bg-card)]"
            >
              {t("projectSelector.create")}
            </button>
          </div>
          <div className="border-t border-[var(--color-border)] px-3 py-2 text-xs">
            <a
              href={lp("/login")}
              className="text-[var(--color-fg-muted)] hover:underline"
            >
              {t("projectSelector.signIn")}
            </a>
          </div>
        </div>
      )}
    </div>
  );
}
