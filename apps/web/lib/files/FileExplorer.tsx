"use client";

// FileExplorer — root-level tree (1-level deep per PR-08).
// Click a file to push it onto the tab bar; drill-down is a
// follow-up.

import { useState } from "react";
import { useT } from "../i18n";
import { useFiles, type FileEntry } from "./useFiles";

interface FileExplorerProps {
  root: string;
  onOpenFile: (path: string, name: string) => void;
}

export function FileExplorer({ root, onOpenFile }: FileExplorerProps) {
  const t = useT();
  const { entries, error, loading } = useFiles(root, 5000);
  const [refreshTick, setRefreshTick] = useState(0);

  if (!root) {
    return (
      <div className="rh-card m-2">
        <p className="text-sm text-[var(--color-fg-muted)]">
          {t("files.noProject")}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2 h-full">
      <header className="flex items-center justify-between">
        <span className="text-xs text-[var(--color-fg-muted)] font-mono truncate">
          {root}
        </span>
        <button
          type="button"
          onClick={() => setRefreshTick((n) => n + 1)}
          aria-label={t("files.refresh")}
          className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] text-xs"
        >
          ↻
        </button>
        {refreshTick === 0 ? null : <span>{refreshTick}</span>}
      </header>
      {loading && entries.length === 0 ? (
        <p className="text-xs text-[var(--color-fg-muted)]">
          {t("common.loading")}
        </p>
      ) : error ? (
        <p role="alert" className="rh-error">
          {error}
        </p>
      ) : entries.length === 0 ? (
        <p className="text-xs text-[var(--color-fg-muted)]">
          {t("files.empty")}
        </p>
      ) : (
        <ul className="flex flex-col gap-0.5 overflow-y-auto">
          {entries.map((e) => (
            <FileRow key={e.name} entry={e} onOpen={onOpenFile} />
          ))}
        </ul>
      )}
    </div>
  );
}

function FileRow({
  entry,
  onOpen,
}: {
  entry: FileEntry;
  onOpen: (path: string, name: string) => void;
}) {
  const t = useT();
  return (
    <li>
      <button
        type="button"
        onClick={() => !entry.is_dir && onOpen(entry.name, entry.name)}
        className={`w-full text-left px-2 py-1 rounded flex items-center gap-2 text-sm ${
          entry.is_dir
            ? "cursor-default text-[var(--color-fg-muted)]"
            : "hover:bg-[var(--color-bg-card)]"
        }`}
        disabled={entry.is_dir}
      >
        <span aria-hidden="true">{entry.is_dir ? "📁" : "📄"}</span>
        <span className="flex-1 truncate font-mono">{entry.name}</span>
        {!entry.is_dir && (
          <span className="text-[10px] text-[var(--color-fg-muted)]">
            {formatSize(entry.size)}
          </span>
        )}
        {entry.is_dir && (
          <span className="text-[10px] text-[var(--color-fg-muted)]" aria-label="directory">
            {t("files.dir")}
          </span>
        )}
      </button>
    </li>
  );
}

function formatSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}
