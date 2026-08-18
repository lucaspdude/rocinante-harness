"use client";

// GitChangesPanel — shows git status for a project root.
// Polls /api/v1/git/status?cwd=... at 5s intervals.

import { useT } from "../i18n";
import { useGitStatus } from "./useFiles";

interface Props {
  cwd: string | null;
}

export function GitChangesPanel({ cwd }: Props) {
  const t = useT();
  const { files, clean, loading, error } = useGitStatus(cwd, 5000);

  if (!cwd) {
    return (
      <p className="text-xs text-[var(--color-fg-muted)] p-2">
        {t("files.noProject")}
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-2 h-full p-2">
      <header className="flex items-center justify-between">
        <span className="text-xs font-medium uppercase tracking-wide">
          {t("files.changes")}
        </span>
        <span className="text-xs text-[var(--color-fg-muted)]">
          {clean ? t("files.clean") : t("files.dirty", { count: files.length })}
        </span>
      </header>
      {error ? (
        <p role="alert" className="rh-error">
          {error}
        </p>
      ) : loading && files.length === 0 ? (
        <p className="text-xs text-[var(--color-fg-muted)]">{t("common.loading")}</p>
      ) : files.length === 0 ? (
        <p className="text-xs text-[var(--color-fg-muted)]">
          {t("files.changesEmpty")}
        </p>
      ) : (
        <ul className="flex flex-col gap-0.5 overflow-y-auto">
          {files.map((f) => (
            <li
              key={f.path}
              className="flex items-center gap-2 px-2 py-1 rounded hover:bg-[var(--color-bg-card)] text-sm"
            >
              <span className="font-mono text-xs text-[var(--color-fg-muted)]">
                {f.status}
              </span>
              <span className="font-mono truncate flex-1">{f.path}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
