"use client";

// Reusable Providers panel. Used by:
//   - Settings → Providers tab
//   - Onboarding step 1 (gate before passphrase init)
//
// Renders the canonical provider list (from /api/v1/meta) with a
// search box. Configured providers float to the top. Each one has
// an inline form: the user pastes their API key, clicks Save, and
// the api writes it to its keystore (chmod 0600 file on the api's
// share dir). The api re-reads the keystore on every omp session
// spawn, so the new key is picked up by the next prompt without
// any process restart. Existing keys can be removed with a Clear
// button.
//
// PR-01 reshape: providers are now a flat array of ProviderInfo
// (id + name + auth + authenticated + help_url). The previous 5-
// provider hardcoded list is gone.

import { useEffect, useMemo, useState } from "react";
import { useT } from "../i18n";
import { useProviders, type ProviderInfo } from "./useProviders";

export function ProvidersPanel({
  onConfiguredCountChange,
}: {
  onConfiguredCountChange?: (count: number) => void;
}) {
  const t = useT();
  const { providers, error, reload, saveKey, deleteKey, saving } =
    useProviders(5000);
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  useEffect(() => {
    if (!onConfiguredCountChange) return;
    onConfiguredCountChange(providers.filter((p) => p.authenticated).length);
  }, [providers, onConfiguredCountChange]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    const list = q
      ? providers.filter(
          (p) =>
            p.id.toLowerCase().includes(q) ||
            p.name.toLowerCase().includes(q) ||
            ((p.env_vars?.[0] ?? p.id).toLowerCase().includes(q))
        )
      : providers;
    return [...list].sort((a, b) => {
      if (a.authenticated !== b.authenticated) {
        return a.authenticated ? -1 : 1;
      }
      return a.name.localeCompare(b.name);
    });
  }, [providers, search]);

  function startEdit(p: ProviderInfo) {
    setEditing(p.id);
    setDraft("");
    setLocalError(null);
  }

  function cancelEdit() {
    setEditing(null);
    setDraft("");
    setLocalError(null);
  }

  async function save(p: ProviderInfo) {
    if (!draft.trim()) {
      setLocalError("empty");
      return;
    }
    setLocalError(null);
    try {
      await saveKey(p.id, draft.trim());
      setEditing(null);
      setDraft("");
      reload();
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : String(e));
    }
  }

  async function clear(p: ProviderInfo) {
    setLocalError(null);
    try {
      await deleteKey(p.id);
      reload();
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="rh-card">
        <h3 className="text-sm font-medium mb-3">
          {t("providers.checklist")}
        </h3>
        {(error || localError) && (
          <p role="alert" className="rh-error mb-3">
            {localError ?? error}
          </p>
        )}
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("providers.search")}
          className="rh-input mb-3 text-sm"
          aria-label={t("providers.search")}
        />
        {filtered.length === 0 ? (
          <p className="text-sm text-[var(--color-fg-muted)]">
            {t("providers.empty")}
          </p>
        ) : (
          <ul className="flex flex-col gap-3">
            {filtered.map((p) => (
              <ProviderRow
                key={p.id}
                p={p}
                editing={editing === p.id}
                draft={draft}
                onDraftChange={setDraft}
                onStartEdit={() => startEdit(p)}
                onCancel={cancelEdit}
                onSave={() => save(p)}
                onClear={() => clear(p)}
                saving={saving === p.id}
              />
            ))}
          </ul>
        )}
      </div>

      <div className="text-xs text-[var(--color-fg-muted)]">
        {t("providers.envReloadNote")}
      </div>
    </div>
  );
}

function ProviderRow({
  p,
  editing,
  draft,
  onDraftChange,
  onStartEdit,
  onCancel,
  onSave,
  onClear,
  saving,
}: {
  p: ProviderInfo;
  editing: boolean;
  draft: string;
  onDraftChange: (v: string) => void;
  onStartEdit: () => void;
  onCancel: () => void;
  onSave: () => void;
  onClear: () => void;
  saving: boolean;
}) {
  const t = useT();
  return (
    <li className="flex flex-col gap-2 py-2 border-b border-[var(--color-border)] last:border-0">
      <div className="flex items-center gap-3">
        <span
          aria-hidden="true"
          className={
            p.authenticated
              ? "inline-block w-2.5 h-2.5 rounded-full bg-green-500"
              : "inline-block w-2.5 h-2.5 rounded-full bg-zinc-500/40"
          }
        />
        <div className="flex-1 min-w-0">
          <div className="font-medium flex flex-col gap-1">
            <div className="flex items-center gap-2">
              {p.name}
              {p.help_url && (
                <a
                  href={p.help_url}
                  target="_blank"
                  rel="noreferrer noopener"
                  className="text-xs text-[var(--color-fg-muted)] hover:underline"
                >
                  {t("providers.helpLink")}
                </a>
              )}
              <span className="text-xs px-1.5 py-0.5 rounded bg-zinc-500/10 text-[var(--color-fg-muted)]">
                {p.supports_login ? "/login" : p.keyless ? "keyless" : "paste-key"}
              </span>
            </div>
            {(p.env_vars ?? []).map((env) => (
              <div key={env} className="text-xs text-[var(--color-fg-muted)] font-mono">
                {env}
              </div>
            ))}
          </div>
        </div>
        <span
          className={
            p.authenticated
              ? "text-xs px-2 py-0.5 rounded-full bg-green-500/15 text-green-600 dark:text-green-400"
              : "text-xs px-2 py-0.5 rounded-full bg-zinc-500/10 text-[var(--color-fg-muted)]"
          }
        >
          {p.authenticated ? t("providers.configured") : t("providers.missing")}
        </span>
      </div>

      {editing ? (
        <div className="flex flex-col gap-2 pl-6">
          <input
            type="password"
            autoComplete="off"
            value={draft}
            onChange={(e) => onDraftChange(e.target.value)}
            placeholder={`${p.env_vars?.[0] ?? p.id}=...`}
            className="rh-input font-mono text-sm"
            disabled={saving}
          />
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onSave}
              disabled={saving}
              className="rh-button-primary text-sm"
            >
              {saving ? t("common.loading") : t("common.save")}
            </button>
            <button
              type="button"
              onClick={onCancel}
              disabled={saving}
              className="rh-button-ghost text-sm"
            >
              {t("common.cancel")}
            </button>
          </div>
        </div>
      ) : (
        <div className="flex gap-2 pl-6">
          {p.authenticated ? (
            <button
              type="button"
              onClick={onClear}
              disabled={saving}
              className="rh-button-ghost text-sm"
            >
              {t("providers.clearKey")}
            </button>
          ) : (
            <button
              type="button"
              onClick={onStartEdit}
              disabled={saving}
              className="rh-button-primary text-sm"
            >
              {t("providers.setKey")}
            </button>
          )}
        </div>
      )}
    </li>
  );
}
