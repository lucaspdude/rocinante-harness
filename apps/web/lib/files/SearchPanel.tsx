"use client";

// SearchPanel — debounced search across the project tree. PR-06.
//
// Wires `useSearch` (POST /api/v1/search) to a small form: pattern
// input, regex toggle, file-glob filter. Clicking a result opens
// the file in the existing file viewer via the `onOpenFile`
// callback supplied by RightSidebar.

import { useEffect, useRef, useState } from "react";
import { useT } from "../i18n";
import { useToast } from "../toast";
import { useSearch, type SearchMatch, type SearchOptions } from "./useSearch";

interface SearchPanelProps {
  root: string | null;
  onOpenFile?: (path: string, name: string) => void;
}

const DEFAULT_OPTIONS: SearchOptions = {
  regex: false,
  maxResults: 200,
  caseSensitive: false,
  fileGlob: "",
};

export function SearchPanel({ root, onOpenFile }: SearchPanelProps) {
  const t = useT();
  const toast = useToast();
  const [pattern, setPattern] = useState("");
  const [opts, setOpts] = useState<SearchOptions>(DEFAULT_OPTIONS);
  const { results, partial, loading, error, code } = useSearch(
    root ?? "",
    pattern,
    opts
  );
  const lastErrorRef = useRef<string | null>(null);

  // Surface errors once per change (mirrors GitChangesPanel /
  // FileViewer pattern).
  useEffect(() => {
    if (error && error !== lastErrorRef.current) {
      lastErrorRef.current = error;
      toast.error(error);
    }
    if (!error) lastErrorRef.current = null;
  }, [error, code, toast]);

  if (!root) {
    return (
      <p className="text-xs text-[var(--color-fg-muted)] p-2">
        {t("files.noProject")}
      </p>
    );
  }

  function basename(p: string): string {
    const idx = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
    return idx >= 0 ? p.slice(idx + 1) : p;
  }

  function handleOpen(m: SearchMatch) {
    if (onOpenFile) {
      onOpenFile(m.path, basename(m.path));
      return;
    }
    // Fallback: fire a toast so the user sees the click landed.
    toast.info(
      t("search.openResult", {
        path: m.path,
        line: String(m.line),
        match: m.match,
      })
    );
  }

  const count = results.length;
  const placeholder = t("search.placeholder", { root: basename(root) || root });

  return (
    <div className="flex flex-col gap-2 h-full p-2">
      <header className="flex flex-col gap-2">
        <input
          type="text"
          value={pattern}
          onChange={(e) => setPattern(e.target.value)}
          placeholder={placeholder}
          aria-label={placeholder}
          className="w-full px-2 py-1 text-sm rounded border border-[var(--color-border)] bg-[var(--color-bg)] outline-none focus:border-blue-500"
          spellCheck={false}
          autoComplete="off"
        />
        <div className="flex items-center gap-2 flex-wrap">
          <label className="flex items-center gap-1 text-xs text-[var(--color-fg-muted)]">
            <input
              type="checkbox"
              checked={opts.regex}
              onChange={(e) => setOpts({ ...opts, regex: e.target.checked })}
            />
            {t("search.regexLabel")}
          </label>
          <input
            type="text"
            value={opts.fileGlob}
            onChange={(e) => setOpts({ ...opts, fileGlob: e.target.value })}
            placeholder={t("search.globLabel")}
            aria-label={t("search.globLabel")}
            className="flex-1 min-w-[8rem] px-2 py-1 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg)] outline-none focus:border-blue-500"
            spellCheck={false}
            autoComplete="off"
          />
        </div>
      </header>

      <div className="flex items-center justify-between text-xs text-[var(--color-fg-muted)]">
        <span>
          {loading
            ? t("common.loading")
            : pattern
              ? t("search.matchCount", { count })
              : ""}
        </span>
        {partial && (
          <span className="text-yellow-600 dark:text-yellow-400">
            {t("search.partial")}
          </span>
        )}
      </div>

      {pattern && !loading && results.length === 0 ? (
        <p className="text-xs text-[var(--color-fg-muted)]">{t("search.empty")}</p>
      ) : (
        <ul className="flex flex-col gap-0.5 overflow-y-auto">
          {results.map((m, i) => (
            <li key={`${m.path}:${m.line}:${m.column}:${i}`}>
              <button
                type="button"
                onClick={() => handleOpen(m)}
                className="w-full text-left px-2 py-1 rounded hover:bg-[var(--color-bg-card)] text-sm flex flex-col gap-0.5"
              >
                <span className="font-mono text-xs text-[var(--color-fg-muted)] truncate">
                  {m.path}:{m.line}
                </span>
                <span className="font-mono text-xs truncate">
                  <span className="bg-yellow-200/40 dark:bg-yellow-500/20 rounded px-0.5">
                    {m.match}
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
