"use client";

// SshServerPanel — Settings → Developer tools → SSH Servers.
//
// Card list of registered remote connections. Each card shows
// label, user@host:port, and the last test outcome badge. The
// "Add a server" button opens a modal with a form (label, host,
// port, username, key dropdown). The "Test now" button fires
// POST /api/v1/ssh/servers/{id}/test and renders the classified
// outcome (ok / auth_failed / conn_refused / network /
// not_installed). The "Remove" button has a confirmation dialog.
//
// The key dropdown reads from the same useSshKeys hook that
// powers GitSshPanel — PR-04 already lists keys for the Git
// providers, so we reuse them here. When no keys exist yet the
// modal shows a "Create a key first" hint rather than an empty
// dropdown, so the user is nudged to GitSshPanel first.

import { useEffect, useMemo, useState } from "react";
import { useT } from "../i18n";
import { useSshKeys } from "./useSshKeys";
import {
  useSshServers,
  type SshServer,
  type TestOutcome,
  type TestResponse,
} from "./useSshServers";

export function SshServerPanel() {
  const t = useT();
  const { servers, loading, error, create, remove, test } =
    useSshServers(5000);
  const { keys } = useSshKeys(5000);
  const [adding, setAdding] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<SshServer | null>(null);
  // Per-server last test result. The key is server ID, the value
  // is the outcome the api returned. Cleared by the user clicking
  // "Test now" again.
  const [results, setResults] = useState<Record<string, TestResponse>>({});
  // Server ID currently being tested, so we can disable the
  // button while the api call is in flight.
  const [testing, setTesting] = useState<string | null>(null);

  const keyById = useMemo(() => {
    const map = new Map<string, string>();
    for (const k of keys) map.set(k.id, k.label);
    return map;
  }, [keys]);

  async function runTest(s: SshServer) {
    if (testing) return;
    setTesting(s.id);
    try {
      const res = await test(s.id);
      setResults((prev) => ({ ...prev, [s.id]: res }));
    } finally {
      setTesting(null);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <header>
        <h2 className="text-lg font-medium">{t("sshServers.title")}</h2>
        <p className="text-sm text-[var(--color-fg-muted)] mt-1">
          {t("sshServers.help")}
        </p>
      </header>

      {error && (
        <div className="rh-card border-[var(--color-danger)] text-sm">
          {t("ssh.error", { message: error })}
        </div>
      )}

      <div className="flex items-center justify-end">
        <button
          type="button"
          className="rh-button-primary"
          onClick={() => setAdding(true)}
        >
          {t("sshServers.add")}
        </button>
      </div>

      {servers.length === 0 && !loading && (
        <p className="text-sm text-[var(--color-fg-muted)]">
          {t("sshServers.empty")}
        </p>
      )}

      <div className="flex flex-col gap-3">
        {servers.map((s) => (
          <ServerCard
            key={s.id}
            server={s}
            keyLabel={s.key_id ? keyById.get(s.key_id) ?? s.key_id : ""}
            result={results[s.id]}
            testing={testing === s.id}
            onTest={() => runTest(s)}
            onRemove={() => setConfirmRemove(s)}
          />
        ))}
      </div>

      {adding && (
        <AddServerModal
          keyOptions={keys.map((k) => ({ id: k.id, label: k.label }))}
          onClose={() => setAdding(false)}
          onSubmit={async (args) => {
            await create(args);
            setAdding(false);
          }}
        />
      )}

      {confirmRemove && (
        <RemoveConfirmModal
          server={confirmRemove}
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

function ServerCard({
  server,
  keyLabel,
  result,
  testing,
  onTest,
  onRemove,
}: {
  server: SshServer;
  keyLabel: string;
  result?: TestResponse;
  testing: boolean;
  onTest: () => void;
  onRemove: () => void;
}) {
  const t = useT();
  const badge = renderBadge(t, result);

  return (
    <div className="rh-card flex flex-col gap-2">
      <div className="flex items-center justify-between gap-2">
        <div className="flex flex-col gap-0.5">
          <span className="font-medium">
            {t("sshServers.cardLabel", { label: server.label })}
          </span>
          <span className="text-xs text-[var(--color-fg-muted)]">
            {t("sshServers.cardSubtitle", {
              username: server.username,
              host: server.host,
              port: server.port,
            })}
            {keyLabel && (
              <>
                {" · "}
                <span className="text-[var(--color-fg-muted)]">{keyLabel}</span>
              </>
            )}
          </span>
        </div>
        <div className="flex items-center gap-2">{badge}</div>
      </div>
      <div className="flex items-center gap-2">
        <button
          type="button"
          className="rh-button-ghost"
          onClick={onTest}
          disabled={testing}
        >
          {testing ? "…" : t("sshServers.test")}
        </button>
        <button
          type="button"
          className="rh-button-danger"
          onClick={onRemove}
        >
          {t("sshServers.remove")}
        </button>
      </div>
    </div>
  );
}

function renderBadge(
  t: (key: string, vars?: Record<string, string | number>) => string,
  result?: TestResponse
) {
  if (!result) {
    return (
      <span className="text-xs px-2 py-0.5 rounded border border-[var(--color-border)] text-[var(--color-fg-muted)]">
        —
      </span>
    );
  }
  const { status, message } = result;
  if (status === "ok") {
    return (
      <span
        className="text-xs px-2 py-0.5 rounded border"
        style={{ borderColor: "var(--color-success)", color: "var(--color-success)" }}
        title={message || undefined}
      >
        ✓ {t("sshServers.testOk")}
      </span>
    );
  }
  if (status === "auth_failed") {
    return (
      <span
        className="text-xs px-2 py-0.5 rounded border"
        style={{ borderColor: "var(--color-danger)", color: "var(--color-danger)" }}
        title={message || undefined}
      >
        ✗ {t("sshServers.testAuthFailed")}
      </span>
    );
  }
  if (status === "conn_refused") {
    return (
      <span
        className="text-xs px-2 py-0.5 rounded border"
        style={{ borderColor: "var(--color-warning)", color: "var(--color-warning)" }}
        title={message || undefined}
      >
        ✗ {t("sshServers.testConnRefused")}
      </span>
    );
  }
  if (status === "not_installed") {
    return (
      <span
        className="text-xs px-2 py-0.5 rounded border"
        style={{ borderColor: "var(--color-warning)", color: "var(--color-warning)" }}
        title={message || undefined}
      >
        ! {t("sshServers.testNotInstalled")}
      </span>
    );
  }
  // network (default fallback)
  return (
    <span
      className="text-xs px-2 py-0.5 rounded border"
      style={{ borderColor: "var(--color-warning)", color: "var(--color-warning)" }}
      title={message || undefined}
    >
      ✗ {t("sshServers.testNetwork")}
    </span>
  );
}

function AddServerModal({
  keyOptions,
  onClose,
  onSubmit,
}: {
  keyOptions: { id: string; label: string }[];
  onClose: () => void;
  onSubmit: (args: {
    label: string;
    host: string;
    port: number;
    username: string;
    keyId: string;
  }) => Promise<void>;
}) {
  const t = useT();
  const [label, setLabel] = useState("");
  const [host, setHost] = useState("");
  const [port, setPort] = useState(22);
  const [username, setUsername] = useState("");
  const [keyId, setKeyId] = useState(keyOptions[0]?.id ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const canSubmit =
    label.trim().length > 0 &&
    host.trim().length > 0 &&
    username.trim().length > 0 &&
    port > 0 &&
    port <= 65535;

  async function submit() {
    if (busy || !canSubmit) return;
    setBusy(true);
    setError(null);
    try {
      await onSubmit({
        label: label.trim(),
        host: host.trim(),
        port,
        username: username.trim(),
        keyId,
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setBusy(false);
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t("sshServers.modalTitle")}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="rh-card w-full max-w-md flex flex-col gap-4 max-h-[90vh] overflow-y-auto">
        <header className="flex items-center justify-between">
          <h3 className="text-base font-medium">{t("sshServers.modalTitle")}</h3>
          <button
            type="button"
            className="rh-button-ghost"
            onClick={onClose}
            aria-label="Close"
          >
            ×
          </button>
        </header>

        <div className="flex flex-col gap-3">
          <div>
            <label className="rh-label" htmlFor="server-label">
              {t("sshServers.fieldLabel")}
            </label>
            <input
              id="server-label"
              type="text"
              className="rh-input"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              autoFocus
            />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div className="sm:col-span-2">
              <label className="rh-label" htmlFor="server-host">
                {t("sshServers.fieldHost")}
              </label>
              <input
                id="server-host"
                type="text"
                className="rh-input"
                value={host}
                onChange={(e) => setHost(e.target.value)}
              />
            </div>
            <div>
              <label className="rh-label" htmlFor="server-port">
                {t("sshServers.fieldPort")}
              </label>
              <input
                id="server-port"
                type="number"
                className="rh-input"
                value={port}
                min={1}
                max={65535}
                onChange={(e) => setPort(Number(e.target.value) || 0)}
              />
            </div>
          </div>
          <div>
            <label className="rh-label" htmlFor="server-user">
              {t("sshServers.fieldUser")}
            </label>
            <input
              id="server-user"
              type="text"
              className="rh-input"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </div>
          <div>
            <label className="rh-label" htmlFor="server-key">
              {t("sshServers.fieldKey")}
            </label>
            {keyOptions.length === 0 ? (
              <p className="text-xs text-[var(--color-fg-muted)]">
                {t("sshServers.empty")}
              </p>
            ) : (
              <select
                id="server-key"
                className="rh-input"
                value={keyId}
                onChange={(e) => setKeyId(e.target.value)}
              >
                {keyOptions.map((k) => (
                  <option key={k.id} value={k.id}>
                    {k.label}
                  </option>
                ))}
              </select>
            )}
          </div>
        </div>

        {error && (
          <p className="text-sm text-[var(--color-danger)]">{error}</p>
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
            disabled={busy || !canSubmit}
          >
            {busy ? "…" : t("sshServers.add")}
          </button>
        </footer>
      </div>
    </div>
  );
}

function RemoveConfirmModal({
  server,
  onClose,
  onConfirm,
}: {
  server: SshServer;
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
      aria-label={t("sshServers.remove")}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="rh-card w-full max-w-md flex flex-col gap-4">
        <h3 className="text-base font-medium">{t("sshServers.remove")}</h3>
        <p className="text-sm">
          {t("sshServers.removeConfirm", { label: server.label })}
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
            {busy ? "…" : t("sshServers.remove")}
          </button>
        </footer>
      </div>
    </div>
  );
}

// TestOutcome is re-exported so the panel can be type-narrowed
// from consuming code without re-importing the hook.
export type { TestOutcome };
