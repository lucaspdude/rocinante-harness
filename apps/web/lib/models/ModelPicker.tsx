"use client";

// ModelPicker — PR-02 dropdown that searches models.dev via
// /api/v1/models/catalog and lets the user pick a model id (sent
// to the api on session create).
//
// Review followup (10-review.md F5): renders the additional
// fields introduced in ModelsDevEntry (max_tokens, cache cost,
// reasoning, thinking_supported, auth_supported).

import { useEffect, useRef, useState } from "react";
import { useT } from "../i18n";
import { useModelCatalog, type ModelEntry } from "./useModelCatalog";

interface ModelPickerProps {
  value: string;
  onChange: (modelId: string) => void;
}

export function ModelPicker({ value, onChange }: ModelPickerProps) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const { models, loading, stale } = useModelCatalog(query);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function clickAway(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    if (open) {
      document.addEventListener("mousedown", clickAway);
      return () => document.removeEventListener("mousedown", clickAway);
    }
    return;
  }, [open]);

  function pick(m: ModelEntry) {
    onChange(m.id);
    setOpen(false);
    setQuery("");
  }

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
        className="rh-button-ghost text-xs w-full truncate text-left"
      >
        {value ? (
          <span className="font-mono">{value}</span>
        ) : (
          <span className="text-[var(--color-fg-muted)]">
            {t("composer.modelPlaceholder")}
          </span>
        )}
      </button>
      {open && (
        <div className="absolute bottom-full mb-2 left-0 right-0 max-h-80 overflow-hidden rh-card z-10 flex flex-col">
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("composer.modelSearch")}
            aria-label={t("composer.modelSearch")}
            className="rh-input text-sm mb-2"
            autoFocus
          />
          {stale && (
            <p className="text-xs text-[var(--color-fg-muted)] mb-1">
              {t("composer.modelStale")}
            </p>
          )}
          {loading && (
            <p className="text-xs text-[var(--color-fg-muted)]">
              {t("common.loading")}
            </p>
          )}
          {!loading && models.length === 0 && (
            <p className="text-xs text-[var(--color-fg-muted)]">
              {t("composer.modelEmpty")}
            </p>
          )}
          {!loading && models.length > 0 && (
            <ul role="listbox" className="overflow-y-auto max-h-60">
              {models.map((m) => (
                <li
                  key={`${m.provider}:${m.id}`}
                  className="flex flex-col gap-0.5 px-2 py-1.5 rounded hover:bg-[var(--color-bg-card)] cursor-pointer"
                  onClick={() => pick(m)}
                  role="option"
                  aria-selected={m.id === value}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-sm truncate">{m.id}</span>
                    <span className="text-xs text-[var(--color-fg-muted)]">
                      {m.provider}
                    </span>
                  </div>
                  {m.name && m.name !== m.id && (
                    <span className="text-xs text-[var(--color-fg-muted)] truncate">
                      {m.name}
                    </span>
                  )}
                  {m.context_length ? (
                    <span className="text-xs text-[var(--color-fg-muted)]">
                      {Math.round(m.context_length / 1000)}K ctx
                    </span>
                  ) : null}
                  <div className="flex flex-wrap items-center gap-1 text-[10px] text-[var(--color-fg-muted)]">
                    {m.reasoning ? (
                      <span
                        title="Supports reasoning"
                        className="px-1 py-0.5 rounded bg-blue-500/10 text-blue-600 dark:text-blue-400"
                      >
                        reasoning
                      </span>
                    ) : null}
                    {m.thinking_supported ? (
                      <span
                        title="Supports thinking"
                        className="px-1 py-0.5 rounded bg-purple-500/10 text-purple-600 dark:text-purple-400"
                      >
                        thinking
                      </span>
                    ) : null}
                    {m.cost_cache_read != null || m.cost_cache_write != null ? (
                      <span
                        title="Has cache pricing"
                        className="px-1 py-0.5 rounded bg-amber-500/10 text-amber-700 dark:text-amber-300"
                      >
                        cache {(m.cost_cache_read ?? 0).toFixed(2)}/{(m.cost_cache_write ?? 0).toFixed(2)}
                      </span>
                    ) : null}
                    {m.auth_supported ? (
                      <span
                        title="Supports /login auth"
                        className="px-1 py-0.5 rounded bg-green-500/10 text-green-600 dark:text-green-400"
                      >
                        /login
                      </span>
                    ) : null}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
