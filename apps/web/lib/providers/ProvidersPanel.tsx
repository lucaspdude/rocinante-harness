"use client"

// Reusable Providers panel. Used by:
//   - Settings → Providers tab
//   - Onboarding step 2 (after passphrase)
//
// Renders a checklist of the 5 supported providers, indicating
// which ones are configured (env var set in the api process).
// Below the checklist, each provider has an inline form: the
// user pastes their API key, clicks Save, and the api writes it
// to its keystore (chmod 0600 file on the api's share dir). The
// api re-reads the keystore on every omp session spawn, so the
// new key is picked up by the next prompt without any process
// restart. Existing keys can be removed with a Clear button.

import { useState } from "react";
import { useT } from "../i18n";
import {
  PROVIDERS,
  useProviders,
  type ProviderDef,
  type ProviderStatus,
} from "./useProviders";

export function ProvidersPanel() {
  const t = useT();
  const { status, error, reload, saveKey, deleteKey, saving } =
    useProviders(5000);
  const [editing, setEditing] = useState<ProviderDef["key"] | null>(null);
  const [draft, setDraft] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);

  function startEdit(p: ProviderDef) {
    setEditing(p.key);
    setDraft("");
    setLocalError(null);
  }

  function cancelEdit() {
    setEditing(null);
    setDraft("");
    setLocalError(null);
  }

  async function save(p: ProviderDef) {
    if (!draft.trim()) {
      setLocalError("empty");
      return;
    }
    setLocalError(null);
    try {
      await saveKey(p.key, draft.trim());
      setEditing(null);
      setDraft("");
      // The 5 s poll will also catch up, but a manual reload
      // gives instant feedback in the UI.
      reload();
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : String(e));
    }
  }

  async function clear(p: ProviderDef) {
    setLocalError(null);
    try {
      await deleteKey(p.key);
      reload();
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h2 className="text-lg font-medium mb-1">{t("providers.title")}</h2>
        <p className="text-sm text-[var(--color-fg-muted)]">
          {t("providers.subtitle")}
        </p>
      </div>

      <div className="rh-card">
        <h3 className="text-sm font-medium mb-3">
          {t("providers.checklist")}
        </h3>
        {(error || localError) && (
          <p role="alert" className="rh-error mb-3">
            {localError ?? error}
          </p>
        )}
        <ul className="flex flex-col gap-3">
          {PROVIDERS.map((p) => (
            <ProviderRow
              key={p.key}
              p={p}
              configured={status?.[p.key] ?? false}
              editing={editing === p.key}
              draft={draft}
              onDraftChange={setDraft}
              onStartEdit={() => startEdit(p)}
              onCancel={cancelEdit}
              onSave={() => save(p)}
              onClear={() => clear(p)}
              saving={saving === p.key}
            />
          ))}
        </ul>
      </div>

      <div className="text-xs text-[var(--color-fg-muted)]">
        {t("providers.envReloadNote")}
      </div>
    </div>
  );
}

function ProviderRow({
  p,
  configured,
  editing,
  draft,
  onDraftChange,
  onStartEdit,
  onCancel,
  onSave,
  onClear,
  saving,
}: {
  p: ProviderDef;
  configured: boolean;
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
            configured
              ? "inline-block w-2.5 h-2.5 rounded-full bg-green-500"
              : "inline-block w-2.5 h-2.5 rounded-full bg-zinc-500/40"
          }
        />
        <div className="flex-1 min-w-0">
          <div className="font-medium flex items-center gap-2">
            {p.label}
            <a
              href={p.helpUrl}
              target="_blank"
              rel="noreferrer noopener"
              className="text-xs text-[var(--color-fg-muted)] hover:underline"
            >
              ({p.installHint})
            </a>
          </div>
          <div className="text-xs text-[var(--color-fg-muted)] font-mono">
            {p.envVar}
          </div>
        </div>
        <span
          className={
            configured
              ? "text-xs px-2 py-0.5 rounded-full bg-green-500/15 text-green-600 dark:text-green-400"
              : "text-xs px-2 py-0.5 rounded-full bg-zinc-500/10 text-[var(--color-fg-muted)]"
          }
        >
          {configured ? t("providers.configured") : t("providers.missing")}
        </span>
      </div>

      {editing ? (
        <div className="flex flex-col gap-2 pl-6">
          <input
            type="password"
            autoComplete="off"
            value={draft}
            onChange={(e) => onDraftChange(e.target.value)}
            placeholder={`${p.envVar}=...`}
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
          {configured ? (
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
