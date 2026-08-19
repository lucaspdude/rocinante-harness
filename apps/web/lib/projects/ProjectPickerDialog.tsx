"use client";

// ProjectPickerDialog — opened by the new-session flow when the
// user has registered projects.

import { useEffect, useState } from "react";
import { useT } from "../i18n";
import { useProjects, type Project } from "./useProjects";

interface Props {
  open: boolean;
  onClose: () => void;
  onPick: (project: Project) => void;
}

export function ProjectPickerDialog({ open, onClose, onPick }: Props) {
  const t = useT();
  const { projects, loading } = useProjects(5000, open);
  const [query, setQuery] = useState("");

  useEffect(() => {
    if (!open) setQuery("");
  }, [open]);

  if (!open) return null;

  const filtered = projects.filter(
    (p) =>
      !query.trim() ||
      p.path.toLowerCase().includes(query.toLowerCase()) ||
      p.name.toLowerCase().includes(query.toLowerCase())
  );

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="project-picker-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
    >
      <div className="rh-card w-full max-w-lg max-h-[90vh] flex flex-col overflow-hidden">
        <header className="flex items-center justify-between mb-3">
          <h2 id="project-picker-title" className="text-base font-medium">
            {t("projects.picker.title")}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label={t("common.close")}
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            ×
          </button>
        </header>

        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("projects.picker.search")}
          aria-label={t("projects.picker.search")}
          className="rh-input text-sm mb-3"
          autoFocus
        />

        {loading && projects.length === 0 ? (
          <p className="text-sm text-[var(--color-fg-muted)]">
            {t("common.loading")}
          </p>
        ) : filtered.length === 0 ? (
          <p className="text-sm text-[var(--color-fg-muted)]">
            {t("projects.picker.empty")}
          </p>
        ) : (
          <ul role="listbox" className="overflow-y-auto max-h-80">
            {filtered.map((p) => (
              <li key={p.path}>
                <button
                  type="button"
                  onClick={() => onPick(p)}
                  className="w-full text-left flex flex-col gap-1 px-3 py-2 rounded hover:bg-[var(--color-bg-card)]"
                >
                  <span className="font-medium">{p.name}</span>
                  <span className="text-xs text-[var(--color-fg-muted)] font-mono truncate">
                    {p.path}
                  </span>
                  <span className="text-xs text-[var(--color-fg-muted)]">
                    {t("projects.picker.sessionCount", {
                      count: p.session_count,
                    })}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}

        <footer className="mt-auto pt-3 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="rh-button-ghost text-sm"
          >
            {t("common.close")}
          </button>
        </footer>
      </div>
    </div>
  );
}
