"use client";

// CliToolsPanel — Settings → Developer tools → CLI tools.
//
// Renders two cards (Azure CLI, GitHub CLI). Each card has:
//
//   - Status badge (Installed / Not installed / Signed in /
//     Authenticated as nobody / Error).
//   - "Install" / "Sign in" button.
//   - On Linux: shows a hint "install yourself; this panel
//     will detect it" (per OQ5 in the plan — macOS-only
//     install recipes for v2).
//
// Install flow:
//   1. User clicks Install → modal opens.
//   2. Modal opens an EventSource to /api/v1/cli-tools/{id}/install/{jobId}/stream.
//   3. SSE events (log / status / end) populate a scrolling
//      log buffer in the modal.
//
// Login flow:
//   1. User clicks Sign in → modal opens.
//   2. Modal POSTs /api/v1/cli-tools/{id}/login/start and
//      receives { jobId, authUrl, authCode }.
//   3. User clicks "Open the URL" in a new tab and types the
//      code on the provider's sign-in page.
//   4. User pastes the code into the modal's input → modal
//      POSTs /api/v1/cli-tools/{id}/login/{jobId}/ack with
//      the value.
//
// PR-40 lesson is honoured by passing intervalMs=0 to
// useCliStatuses when the panel is hidden (the parent
// controls this via the `active` prop).

import { useCallback, useEffect, useRef, useState } from "react";
import { useT } from "../i18n";
import {
  useCliStatuses,
  startInstall,
  startLogin,
  ackLogin,
  openInstallStream,
  type CliSpec,
  type CliStatus,
} from "./useCliTools";

interface Props {
  active?: boolean;
}

interface InstallModalProps {
  spec: CliSpec;
  onClose: () => void;
  onComplete: () => void;
}

function InstallModal({ spec, onClose, onComplete }: InstallModalProps) {
  const t = useT();
  const [lines, setLines] = useState<string[]>([]);
  const [status, setStatus] = useState<"running" | "done" | "failed">(
    "running"
  );
  const [exitCode, setExitCode] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const closeRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function go() {
      try {
        const { jobId } = await startInstall(spec.id);
        if (cancelled) return;
        closeRef.current = openInstallStream(spec.id, jobId, {
          onLog: (line) =>
            setLines((prev) => {
              const next = [...prev, line];
              // cap at 200 (matches the api's ringCap)
              return next.length > 200 ? next.slice(-200) : next;
            }),
          onStatus: (s) =>
            setStatus(s === "done" ? "done" : s === "failed" ? "failed" : "running"),
          onEnd: (s, code) => {
            setStatus(s === "done" ? "done" : "failed");
            setExitCode(code);
            onComplete();
          },
          onError: (err) => setError(err.message),
        });
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
        setStatus("failed");
      }
    }
    go();
    return () => {
      cancelled = true;
      if (closeRef.current) closeRef.current();
    };
  }, [spec.id, onComplete]);

  // Auto-scroll to bottom when new lines arrive.
  const logRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [lines]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={`Install ${spec.displayName}`}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="rh-card w-full max-w-2xl flex flex-col gap-3 max-h-[90vh] overflow-hidden">
        <header className="flex items-center justify-between">
          <h3 className="text-base font-medium">
            {t("clis.logStream")} — {spec.displayName}
          </h3>
          <button
            type="button"
            className="rh-button-ghost"
            onClick={onClose}
            aria-label={t("clis.close")}
          >
            ×
          </button>
        </header>
        <div
          ref={logRef}
          className="bg-black text-green-300 font-mono text-xs rounded p-3 overflow-y-auto flex-1 min-h-[200px] max-h-[60vh]"
        >
          {lines.length === 0 ? (
            <span className="text-green-700">
              {status === "running"
                ? t("clis.installing")
                : status === "done"
                  ? t("clis.installOk")
                  : t("clis.signInFailed")}
            </span>
          ) : (
            lines.map((line, i) => (
              <div key={i} className="whitespace-pre-wrap break-words">
                {line}
              </div>
            ))
          )}
        </div>
        <footer className="flex items-center justify-between text-xs text-[var(--color-fg-muted)]">
          <span>
            {t("clis.statusBadge")}: {status}
            {exitCode !== null ? ` (${exitCode})` : ""}
          </span>
          {error && <span className="text-[var(--color-danger)]">{error}</span>}
          {status !== "running" && (
            <button
              type="button"
              className="rh-button-ghost"
              onClick={onClose}
            >
              {t("clis.close")}
            </button>
          )}
        </footer>
      </div>
    </div>
  );
}

interface LoginModalProps {
  spec: CliSpec;
  onClose: () => void;
  onComplete: () => void;
}

function LoginModal({ spec, onClose, onComplete }: LoginModalProps) {
  const t = useT();
  const [jobId, setJobId] = useState<string | null>(null);
  const [authUrl, setAuthUrl] = useState<string | null>(null);
  const [authCode, setAuthCode] = useState<string | null>(null);
  const [codeInput, setCodeInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function go() {
      try {
        const res = await startLogin(spec.id);
        if (cancelled) return;
        setJobId(res.jobId);
        setAuthUrl(res.authUrl ?? null);
        setAuthCode(res.authCode ?? null);
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
      }
    }
    go();
    return () => {
      cancelled = true;
    };
  }, [spec.id]);

  async function copyCode() {
    if (!authCode) return;
    try {
      await navigator.clipboard.writeText(authCode);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Fall back to a manual select; the user can copy from
      // the readonly input.
    }
  }

  async function submit() {
    if (!jobId) return;
    setBusy(true);
    setError(null);
    try {
      await ackLogin(spec.id, jobId, codeInput);
      onComplete();
      onClose();
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
      aria-label={`Sign in ${spec.displayName}`}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="rh-card w-full max-w-md flex flex-col gap-3">
        <header className="flex items-center justify-between">
          <h3 className="text-base font-medium">
            {t("clis.signIn")} — {spec.displayName}
          </h3>
          <button
            type="button"
            className="rh-button-ghost"
            onClick={onClose}
            aria-label={t("clis.close")}
          >
            ×
          </button>
        </header>

        {error && (
          <div className="rh-card border-[var(--color-danger)] text-sm">
            {error}
          </div>
        )}

        {!jobId && !error && (
          <p className="text-sm text-[var(--color-fg-muted)]">
            {t("clis.signInWait")}
          </p>
        )}

        {jobId && (
          <>
            <p className="text-sm">{t("clis.codeHelp")}</p>
            {authUrl && (
              <a
                href={authUrl}
                target="_blank"
                rel="noreferrer"
                className="rh-button-primary self-start"
              >
                {t("clis.openUrl")}
              </a>
            )}
            <div className="flex flex-col gap-1">
              <label className="rh-label">{t("clis.copyCode")}</label>
              <div className="flex items-center gap-2">
                <input
                  className="rh-input font-mono"
                  readOnly
                  value={authCode ?? ""}
                  onFocus={(e) => e.currentTarget.select()}
                />
                <button
                  type="button"
                  className="rh-button-ghost"
                  onClick={copyCode}
                >
                  {copied ? t("clis.copied") : t("clis.copy")}
                </button>
              </div>
            </div>
            <div className="flex flex-col gap-1">
              <label className="rh-label" htmlFor="cli-ack">
                {t("clis.ackPlaceholder")}
              </label>
              <input
                id="cli-ack"
                type="text"
                className="rh-input"
                placeholder={t("clis.ackPlaceholder")}
                value={codeInput}
                onChange={(e) => setCodeInput(e.target.value)}
              />
            </div>
            <button
              type="button"
              className="rh-button-primary"
              disabled={busy || codeInput.length === 0}
              onClick={submit}
            >
              {busy ? t("clis.signInWait") : t("clis.ack")}
            </button>
          </>
        )}
      </div>
    </div>
  );
}

interface CardProps {
  spec: CliSpec;
  status?: CliStatus;
  onInstall: () => void;
  onSignIn: () => void;
  onRefresh: () => void;
}

function CliCard({ spec, status, onInstall, onSignIn, onRefresh }: CardProps) {
  const t = useT();
  const installed = status?.installed ?? false;
  const authenticated = status?.authenticated ?? false;
  const account = status?.account;
  const version = status?.version;

  let badge = t("clis.badgeNotInstalled");
  let badgeClass = "rh-badge rh-badge-muted";
  if (status?.detail && !installed) {
    badge = t("clis.badgeError");
    badgeClass = "rh-badge rh-badge-error";
  } else if (installed && authenticated) {
    badge = account ? `${account}` : t("clis.badgeAuthenticated");
    badgeClass = "rh-badge rh-badge-ok";
  } else if (installed && !authenticated) {
    badge = t("clis.badgeAnonymous");
    badgeClass = "rh-badge rh-badge-warn";
  }

  // Detect Linux so we can show the install-yourself hint.
  const isLinux =
    typeof navigator !== "undefined" &&
    /Linux/i.test(navigator.platform || "");
  const helpKey = spec.id === "az" ? "clis.cardAzHelp" : "clis.cardGhHelp";

  return (
    <div className="rh-card flex flex-col gap-3">
      <header className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h3 className="text-base font-medium">{spec.displayName}</h3>
          <p className="text-xs text-[var(--color-fg-muted)]">
            {t(helpKey)}
          </p>
        </div>
        <span className={badgeClass}>{badge}</span>
      </header>

      {version && (
        <p className="text-xs text-[var(--color-fg-muted)] font-mono">
          {version}
        </p>
      )}

      <div className="flex flex-wrap items-center gap-2">
        {!installed && !isLinux && (
          <button
            type="button"
            className="rh-button-primary"
            onClick={onInstall}
          >
            {t("clis.install")}
          </button>
        )}
        {!installed && isLinux && (
          <p className="text-xs text-[var(--color-fg-muted)]">
            {t("clis.linuxHint", { cli: spec.displayName })}
          </p>
        )}
        {installed && !authenticated && (
          <button
            type="button"
            className="rh-button-primary"
            onClick={onSignIn}
          >
            {t("clis.signIn")}
          </button>
        )}
        {installed && authenticated && (
          <button
            type="button"
            className="rh-button-ghost"
            onClick={onSignIn}
          >
            {t("clis.signIn")}
          </button>
        )}
        <button
          type="button"
          className="rh-button-ghost"
          onClick={onRefresh}
        >
          {t("clis.check")}
        </button>
      </div>

      {status?.detail && !installed && (
        <p className="text-xs text-[var(--color-fg-muted)] font-mono break-words">
          {status.detail}
        </p>
      )}
    </div>
  );
}

export function CliToolsPanel({ active = true }: Props) {
  const t = useT();
  // Pass intervalMs=0 when the panel is hidden so the PR-40
  // setInterval-0 trap can't fire. The hook still returns the
  // last-known statuses, so re-mounting is cheap.
  const { statuses, loading, error, reload } = useCliStatuses(active ? 5000 : 0);
  const [installing, setInstalling] = useState<string | null>(null);
  const [loginCli, setLoginCli] = useState<string | null>(null);

  const specs: CliSpec[] = [
    { id: "az", displayName: "Azure CLI", helpText: t("clis.cardAzHelp") },
    { id: "gh", displayName: "GitHub CLI", helpText: t("clis.cardGhHelp") },
  ];

  const refresh = useCallback(() => {
    reload();
  }, [reload]);

  return (
    <div className="flex flex-col gap-4">
      <header>
        <h2 className="text-lg font-medium">{t("clis.title")}</h2>
        <p className="text-sm text-[var(--color-fg-muted)] mt-1">
          {t("clis.help")}
        </p>
      </header>

      {error && (
        <div className="rh-card border-[var(--color-danger)] text-sm">
          {error}
        </div>
      )}

      <div className="flex flex-col gap-3">
        {specs.map((spec) => (
          <CliCard
            key={spec.id}
            spec={spec}
            status={statuses[spec.id]}
            onInstall={() => setInstalling(spec.id)}
            onSignIn={() => setLoginCli(spec.id)}
            onRefresh={refresh}
          />
        ))}
      </div>

      {loading && statuses["az"] === undefined && (
        <p className="text-sm text-[var(--color-fg-muted)]">…</p>
      )}

      {installing && (
        <InstallModal
          spec={specs.find((s) => s.id === installing)!}
          onClose={() => setInstalling(null)}
          onComplete={refresh}
        />
      )}
      {loginCli && (
        <LoginModal
          spec={specs.find((s) => s.id === loginCli)!}
          onClose={() => setLoginCli(null)}
          onComplete={refresh}
        />
      )}
    </div>
  );
}