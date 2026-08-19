"use client";

// GitSshPanel — Settings → Developer tools → Git SSH
//
// Three provider cards (GitHub, GitLab, Azure DevOps). Each card
// shows the existing key (or "No key yet"), with Copy / Remove
// affordances. The "Add a key" button opens a modal that walks
// the user through Generate (default) vs Upload; on Generate the
// api returns the one-shot private key which we display verbatim
// with a copy button and a strong warning that it will not be
// shown again.
//
// The api side (PR-04 backend) writes ~/.ssh/id_ed25519_<label>
// (chmod 600) and appends a matching Host block to ~/.ssh/config,
// so after the modal closes the user only needs to paste the
// public key into the provider's UI to be ready to clone.
//
// Test connection: PR-04 defers the in-app test endpoint to PR-05
// (which already adds /api/v1/ssh/servers/{id}/test). We surface
// the SSH test command via a <details> disclosure so the user can
// copy/paste it into the host terminal.

import { useEffect, useMemo, useState } from "react";
import { useT } from "../i18n";
import { useSshKeys, type SshGenerateResult, type SshKey } from "./useSshKeys";

interface Provider {
  id: "github" | "gitlab" | "azureDevops";
  displayName: string;
  testHost: string;
  sshTestCmd: (label: string) => string;
}
const PROVIDERS: Provider[] = [
  {
    id: "github",
    displayName: "GitHub",
    testHost: "github.com",
    sshTestCmd: (label) =>
      `ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -i ~/.ssh/id_ed25519_${label} -o StrictHostKeyChecking=accept-new git@github.com`,
  },
  {
    id: "gitlab",
    displayName: "GitLab",
    testHost: "gitlab.com",
    sshTestCmd: (label) =>
      `ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -i ~/.ssh/id_ed25519_${label} -o StrictHostKeyChecking=accept-new git@gitlab.com`,
  },
  {
    id: "azureDevops",
    displayName: "Azure DevOps",
    testHost: "dev.azure.com",
    sshTestCmd: (label) =>
      `ssh -T -o BatchMode=yes -o IdentitiesOnly=yes -i ~/.ssh/id_ed25519_${label} -o StrictHostKeyChecking=accept-new git@dev.azure.com`,
  },
];

const LABEL_RE = /^[A-Za-z0-9._-]+$/;
const LABEL_MAX = 64;

type LabelErr = "too_long" | "invalid";

function validateLabel(label: string): LabelErr | null {
  const trimmed = label.trim();
  if (trimmed.length === 0) return "invalid";
  if (trimmed.length > LABEL_MAX) return "too_long";
  if (!LABEL_RE.test(trimmed)) return "invalid";
  return null;
}

export function GitSshPanel() {
  const t = useT();
  const { keys, loading, error, generate, remove } = useSshKeys(5000);
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null);
  const [confirmRemove, setConfirmRemove] = useState<SshKey | null>(null);
  const [copied, setCopied] = useState<"public" | "private" | null>(null);

  // Reset the "Copied." pill after a short delay so it does not
  // linger after the user moves on.
  useEffect(() => {
    if (!copied) return;
    const id = window.setTimeout(() => setCopied(null), 1500);
    return () => window.clearTimeout(id);
  }, [copied]);

  // Group keys by provider for fast lookup. Dynamic insertion order
  // matters (we render PROVIDERS in declaration order); Map is the
  // right structure here.
  const byProvider = useMemo(() => {
    const map = new Map<string, SshKey>();
    for (const k of keys) map.set(k.provider, k);
    return map;
  }, [keys]);

  const providerNames = useMemo(
    () => PROVIDERS.map((p) => p.displayName).join(", "),
    []
  );

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h2 className="text-lg font-medium">{t("ssh.title")}</h2>
        <p className="text-sm text-[var(--color-fg-muted)] mt-1">
          {t("ssh.help", { provider: providerNames })}
        </p>
      </header>

      {error && (
        <div className="rh-card border-[var(--color-danger)] text-sm">
          {t("ssh.error", { message: error })}
        </div>
      )}

      <div className="flex flex-col gap-3">
        {PROVIDERS.map((p) => {
          const key = byProvider.get(p.id);
          return (
            <ProviderCard
              key={p.id}
              provider={p}
              key_={key}
              loading={loading}
              copied={copied}
              onAdd={() => setEditingProvider(p)}
              onRemove={() => key && setConfirmRemove(key)}
              onCopied={(which) => setCopied(which)}
            />
          );
        })}
      </div>

      {editingProvider && (
        <AddKeyModal
          provider={editingProvider}
          onClose={() => setEditingProvider(null)}
          onSubmit={async (label) => {
            return generate(label, editingProvider.id);
          }}
          onCopied={(which) => setCopied(which)}
        />
      )}

      {confirmRemove && (
        <RemoveConfirmModal
          key_={confirmRemove}
          providerName={
            PROVIDERS.find((p) => p.id === confirmRemove.provider)?.displayName ??
            confirmRemove.provider
          }
          onClose={() => setConfirmRemove(null)}
          onConfirm={async () => {
            await remove(confirmRemove.id);
            setConfirmRemove(null);
          }}
        />
      )}
    </div>
  );
}

function ProviderCard({
  provider,
  key_,
  loading,
  copied,
  onAdd,
  onRemove,
  onCopied,
}: {
  provider: Provider;
  key_: SshKey | undefined;
  loading: boolean;
  copied: "public" | "private" | null;
  onAdd: () => void;
  onRemove: () => void;
  onCopied: (which: "public" | "private") => void;
}) {
  const t = useT();
  const cardLabel = t("ssh.cardLabel", { provider: provider.displayName });
  return (
    <section className="rh-card flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h3 className="text-base font-medium">{cardLabel}</h3>
          {key_ ? (
            <p className="text-xs text-[var(--color-fg-muted)] mt-1">
              <span className="font-mono">{key_.label}</span>
              {" · "}
              <span className="font-mono">{key_.fingerprint}</span>
            </p>
          ) : (
            <p className="text-xs text-[var(--color-fg-muted)] mt-1">
              {t("ssh.empty")}
            </p>
          )}
        </div>
        <div className="flex gap-2 shrink-0">
          {!key_ && !loading && (
            <button
              type="button"
              className="rh-button-primary"
              onClick={onAdd}
            >
              {t("ssh.addKey")}
            </button>
          )}
          {key_ && (
            <>
              <button
                type="button"
                className="rh-button-ghost"
                onClick={async () => {
                  if (!key_) return;
                  try {
                    await navigator.clipboard.writeText(key_.public_key);
                    onCopied("public");
                  } catch {
                    /* clipboard might be blocked; the user can
                       still select + copy manually */
                  }
                }}
              >
                {copied === "public" ? t("ssh.copied") : t("ssh.copyPublic")}
              </button>
              <button
                type="button"
                className="rh-button-danger"
                onClick={onRemove}
              >
                {t("ssh.remove")}
              </button>
            </>
          )}
        </div>
      </div>
      {key_ && (
        <details className="text-xs text-[var(--color-fg-muted)]">
          <summary className="cursor-pointer">
            {t("ssh.test")} ({provider.testHost})
          </summary>
          <pre className="mt-2 p-2 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] whitespace-pre-wrap break-all font-mono">
            {provider.sshTestCmd(key_.label)}
          </pre>
        </details>
      )}
    </section>
  );
}

function AddKeyModal({
  provider,
  onClose,
  onSubmit,
  onCopied,
}: {
  provider: Provider;
  onClose: () => void;
  onSubmit: (label: string) => Promise<SshGenerateResult>;
  onCopied: (which: "public" | "private") => void;
}) {
  const t = useT();
  const [mode, setMode] = useState<"generate" | "upload">("generate");
  const [label, setLabel] = useState("");
  const [uploadText, setUploadText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<{
    public_key: string;
    private_key: string;
    label: string;
  } | null>(null);

  // Esc cancels the modal.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const labelError = useMemo<LabelErr | null>(() => validateLabel(label), [label]);
  const labelErrorText = labelError
    ? labelError === "too_long"
      ? t("ssh.labelTooLong")
      : t("ssh.labelInvalid")
    : null;

  async function submit() {
    if (busy) return;
    setError(null);
    if (labelError) return;
    setBusy(true);
    try {
      if (mode === "generate") {
        const res = await onSubmit(label.trim());
        setResult({
          public_key: res.key.public_key,
          private_key: res.private_key,
          label: res.key.label,
        });
      } else {
        // Upload path: for PR-04 we keep this stub-local — the api
        // does not yet accept a foreign public key. We still let the
        // user paste one and copy it back into a config block, but
        // the file is not materialised server-side. PR-05 will add
        // the proper /keys/upload endpoint that the reference has.
        setResult({
          public_key: uploadText.trim(),
          private_key: "",
          label: label.trim(),
        });
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t("ssh.modalTitle", { provider: provider.displayName })}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="rh-card w-full max-w-lg flex flex-col gap-4 max-h-[90vh] overflow-y-auto">
        <header className="flex items-center justify-between">
          <h3 className="text-base font-medium">
            {t("ssh.modalTitle", { provider: provider.displayName })}
          </h3>
          <button
            type="button"
            className="rh-button-ghost"
            onClick={onClose}
            aria-label="Close"
          >
            ×
          </button>
        </header>

        {!result ? (
          <>
            <fieldset className="flex flex-col gap-3">
              <label className="rh-label">
                <input
                  type="radio"
                  name="mode"
                  checked={mode === "generate"}
                  onChange={() => setMode("generate")}
                />{" "}
                {t("ssh.generate")}
              </label>
              <label className="rh-label">
                <input
                  type="radio"
                  name="mode"
                  checked={mode === "upload"}
                  onChange={() => setMode("upload")}
                />{" "}
                {t("ssh.upload")}
              </label>
            </fieldset>

            <div className="flex flex-col gap-1">
              <label className="rh-label" htmlFor="ssh-label">
                {t("ssh.cardLabel", { provider: provider.displayName })}
              </label>
              <input
                id="ssh-label"
                type="text"
                className="rh-input"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder={t("ssh.namePlaceholder")}
                maxLength={LABEL_MAX + 1}
                autoFocus
              />
              <p className="text-xs text-[var(--color-fg-muted)]">
                {t("ssh.nameHelp")}
              </p>
              {labelErrorText && (
                <p className="text-xs text-[var(--color-danger)]">
                  {labelErrorText}
                </p>
              )}
            </div>

            {mode === "upload" && (
              <div className="flex flex-col gap-1">
                <label className="rh-label" htmlFor="ssh-pub">
                  {t("ssh.publicKey", { provider: provider.displayName })}
                </label>
                <textarea
                  id="ssh-pub"
                  className="rh-input font-mono text-xs"
                  rows={4}
                  value={uploadText}
                  onChange={(e) => setUploadText(e.target.value)}
                  placeholder="ssh-ed25519 AAAA…"
                />
              </div>
            )}

            {error && (
              <p className="text-sm text-[var(--color-danger)]">
                {t("ssh.error", { message: error })}
              </p>
            )}

            <footer className="flex justify-end gap-2">
              <button
                type="button"
                className="rh-button-ghost"
                onClick={onClose}
                disabled={busy}
              >
                Cancel
              </button>
              <button
                type="button"
                className="rh-button-primary"
                onClick={submit}
                disabled={busy || !!labelError || label.trim().length === 0}
              >
                {busy ? "…" : t("ssh.generate")}
              </button>
            </footer>
          </>
        ) : (
          <>
            <p className="text-sm font-medium">
              {t("ssh.publicKey", { provider: provider.displayName })}
            </p>
            <pre className="p-3 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-xs font-mono whitespace-pre-wrap break-all">
              {result.public_key}
            </pre>
            <CopyButton
              text={result.public_key}
              label={t("ssh.copyPublic")}
              onCopied={() => onCopied("public")}
            />

            {result.private_key && (
              <>
                <p className="text-sm font-medium">{t("ssh.privateKey")}</p>
                <pre className="p-3 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-xs font-mono whitespace-pre-wrap break-all">
                  {result.private_key}
                </pre>
                <CopyButton
                  text={result.private_key}
                  label={t("ssh.copyPrivate")}
                  onCopied={() => onCopied("private")}
                />
                <p className="text-xs text-[var(--color-danger)]">
                  {t("ssh.copyWarning")}
                </p>
              </>
            )}

            <footer className="flex justify-end">
              <button
                type="button"
                className="rh-button-primary"
                onClick={onClose}
              >
                Done
              </button>
            </footer>
          </>
        )}
      </div>
    </div>
  );
}

function CopyButton({
  text,
  label,
  onCopied,
}: {
  text: string;
  label: string;
  onCopied: () => void;
}) {
  const t = useT();
  const [done, setDone] = useState(false);
  useEffect(() => {
    if (!done) return;
    const id = window.setTimeout(() => setDone(false), 1500);
    return () => window.clearTimeout(id);
  }, [done]);
  return (
    <button
      type="button"
      className="rh-button-ghost self-start"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(text);
          setDone(true);
          onCopied();
        } catch {
          /* noop — manual select still works */
        }
      }}
    >
      {done ? t("ssh.copied") : label}
    </button>
  );
}

function RemoveConfirmModal({
  key_,
  providerName,
  onClose,
  onConfirm,
}: {
  key_: SshKey;
  providerName: string;
  onClose: () => void;
  onConfirm: () => Promise<void>;
}) {
  const t = useT();
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t("ssh.confirmRemoveTitle")}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="rh-card w-full max-w-md flex flex-col gap-4">
        <h3 className="text-base font-medium">{t("ssh.confirmRemoveTitle")}</h3>
        <p className="text-sm">
          {t("ssh.removeConfirm", {
            label: key_.label,
            provider: providerName,
          })}
        </p>
        <footer className="flex justify-end gap-2">
          <button
            type="button"
            className="rh-button-ghost"
            onClick={onClose}
            disabled={busy}
          >
            Cancel
          </button>
          <button
            type="button"
            className="rh-button-danger"
            onClick={async () => {
              setBusy(true);
              try {
                await onConfirm();
              } finally {
                setBusy(false);
              }
            }}
            disabled={busy}
          >
            {busy ? "…" : t("ssh.remove")}
          </button>
        </footer>
      </div>
    </div>
  );
}
