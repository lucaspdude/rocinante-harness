"use client";

// LoginModal — PR-01 unified selector for OAuth + paste-key flows.
//
// Triggered when the user clicks "Sign in with {provider}" for an
// OAuth-style provider. Connects to /api/v1/login/start/{provider}
// (POST), subscribes via EventSource to /api/v1/login/{jobId}/stream,
// and shows each extension_ui_request frame as a step in the
// modal.

import { useEffect, useRef, useState } from "react";
import { useT } from "../i18n";
import { api } from "../api/client";
import { useLoginProviders } from "./useProviders";

interface LoginStartResponse {
  job_id: string;
  stream_url: string;
  status_url: string;
  provider_id: string;
}

interface LoginModalProps {
  open: boolean;
  onClose: () => void;
  onConfigured?: (providerId: string) => void;
  initialProvider?: string;
}

interface StatusFrame {
  state?: string;
  error?: string;
}

interface UIRequestFrame {
  title?: string;
  detail?: string;
}

function getMessageData(ev: Event): string {
  const me = ev as MessageEvent;
  return typeof me.data === "string" ? me.data : "";
}

export function LoginModal({
  open,
  onClose,
  onConfigured,
  initialProvider,
}: LoginModalProps) {
  const t = useT();
  const { providers } = useLoginProviders(open ? 5000 : 0);
  const [providerId, setProviderId] = useState(initialProvider ?? "");
  const [phase, setPhase] = useState<
    "idle" | "starting" | "connecting" | "ui" | "complete" | "failed"
  >("idle");
  const [stepTitle, setStepTitle] = useState<string>("");
  const [stepDetail, setStepDetail] = useState<string>("");
  const [errorMsg, setErrorMsg] = useState<string>("");
  const [inputValue, setInputValue] = useState("");
  const [events, setEvents] = useState<{ event: string; data: string }[]>([]);
  const eventSourceRef = useRef<EventSource | null>(null);
  const jobIdRef = useRef<string>("");

  function closeEventSource() {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }

  async function startLogin(idOverride?: string) {
    const id = idOverride ?? providerId;
    if (!id) return;
    setPhase("starting");
    setErrorMsg("");
    setStepTitle("");
    setStepDetail("");
    setEvents([]);
    try {
      const res = await api.post<LoginStartResponse>(
        `/api/v1/login/start/${encodeURIComponent(id)}`,
        { unauthenticated: true }
      );
      jobIdRef.current = res.job_id;
      setPhase("connecting");
      const es = new EventSource(res.stream_url);
      eventSourceRef.current = es;

      const append = (eventName: string) => (ev: Event) => {
        const data = getMessageData(ev);
        setEvents((cur) => [...cur, { event: eventName, data }]);
      };
      es.addEventListener("spawn", append("spawn"));
      es.addEventListener("ui_request", (ev) => {
        const data = getMessageData(ev);
        setEvents((cur) => [...cur, { event: "ui_request", data }]);
        let parsed: UIRequestFrame = {};
        try {
          parsed = JSON.parse(data);
        } catch {
          // ignore malformed frames
        }
        if (typeof parsed.title === "string") setStepTitle(parsed.title);
        if (typeof parsed.detail === "string") setStepDetail(parsed.detail);
        setPhase("ui");
      });
      es.addEventListener("ack", append("ack"));
      es.addEventListener("log", append("log"));
      es.addEventListener("status", (ev) => {
        const data = getMessageData(ev);
        setEvents((cur) => [...cur, { event: "status", data }]);
        let parsed: StatusFrame = {};
        try {
          parsed = JSON.parse(data);
        } catch {
          // ignore
        }
        if (parsed.state === "complete") {
          setPhase("complete");
          onConfigured?.(id);
          closeEventSource();
        } else if (parsed.state === "failed" || parsed.state === "expired") {
          setPhase("failed");
          setErrorMsg(parsed.error ?? "");
          closeEventSource();
        }
      });
      es.onerror = () => {
        // Connection closed by the api (job finished). Phase is
        // already set by the status event; closing is defensive.
        closeEventSource();
      };
    } catch (e: unknown) {
      setPhase("failed");
      const err = e as { body?: { message?: string }; message?: string };
      setErrorMsg(err.body?.message ?? err.message ?? "failed");
    }
  }

  useEffect(() => {
    if (open) {
      if (initialProvider) {
        setProviderId(initialProvider);
        startLogin(initialProvider);
      }
      return;
    }
    closeEventSource();
    setPhase("idle");
    setProviderId(initialProvider ?? "");
    setInputValue("");
    setEvents([]);
    setErrorMsg("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initialProvider]);

  async function sendAck() {
    if (!jobIdRef.current) return;
    await api.post(`/api/v1/login/${jobIdRef.current}/ack`, {
      json: { value: inputValue },
      unauthenticated: true,
    });
    setInputValue("");
  }

  function close() {
    closeEventSource();
    onClose();
  }

  if (!open) return null;

  const configured = providers.filter((p) => p.authenticated);
  const unconfigured = providers.filter((p) => !p.authenticated);
  const currentProvider = providers.find((p) => p.id === providerId);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="login-modal-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
    >
      <div className="rh-card w-full max-w-lg max-h-[90vh] flex flex-col overflow-hidden">
        <header className="flex items-center justify-between mb-3">
          <h2 id="login-modal-title" className="text-base font-medium">
            {t("login.modal.title")}
          </h2>
          <button
            type="button"
            onClick={close}
            aria-label={t("common.close")}
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            ×
          </button>
        </header>

        {providers.length === 0 ? (
          <p className="text-sm text-[var(--color-fg-muted)]">
            {t("login.modal.loading")}
          </p>
        ) : (
          <>
            {!initialProvider && (
              <div className="mb-3">
                <label className="rh-label" htmlFor="login-provider">
                  {t("login.modal.provider")}
                </label>
                <select
                  id="login-provider"
                  className="rh-input"
                  value={providerId}
                  onChange={(e) => setProviderId(e.target.value)}
                  disabled={phase === "starting" || phase === "connecting"}
                >
                  <option value="">{t("login.modal.chooseProvider")}</option>
                  {configured.length > 0 && (
                    <optgroup label={t("login.modal.configured")}>
                      {configured.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name} ✓
                        </option>
                      ))}
                    </optgroup>
                  )}
                  {unconfigured.length > 0 && (
                    <optgroup label={t("login.modal.unconfigured")}>
                      {unconfigured.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name}
                          {p.supports_login && !p.keyless ? ` · /login` : ""}
                          {p.keyless ? ` · keyless` : ""}
                        </option>
                      ))}
                    </optgroup>
                  )}
                </select>
              </div>
            )}
            {currentProvider && (
              <p className="text-xs text-[var(--color-fg-muted)] mb-3">
                {currentProvider.supports_login
                  ? t("login.modal.supportsLogin")
                  : ""}
                {currentProvider.keyless
                  ? t("login.modal.keyless")
                  : ""}
                {!currentProvider.supports_login && !currentProvider.keyless
                  ? t("login.modal.pasteKey")
                  : ""}
              </p>
            )}

            {phase === "idle" && providerId && (
              <button
                type="button"
                onClick={() => startLogin()}
                className="rh-button-primary text-sm mb-3"
              >
                {t("login.modal.start")}
              </button>
            )}

            {(phase === "starting" || phase === "connecting") && (
              <p className="text-sm text-[var(--color-fg-muted)] mb-3">
                {t("login.modal.connecting")}
              </p>
            )}

            {(phase === "ui" || phase === "complete") &&
              (stepTitle || stepDetail) && (
                <div className="bg-[var(--color-bg-card)] rounded p-3 mb-3">
                  {stepTitle && (
                    <p className="font-medium text-sm mb-1">{stepTitle}</p>
                  )}
                  {stepDetail && (
                    <p className="text-xs text-[var(--color-fg-muted)]">
                      {stepDetail}
                    </p>
                  )}
                  {phase === "ui" && (
                    <div className="mt-2 flex flex-col gap-2">
                      <input
                        value={inputValue}
                        onChange={(e) => setInputValue(e.target.value)}
                        placeholder={t("login.modal.inputPlaceholder")}
                        className="rh-input text-sm"
                      />
                      <button
                        type="button"
                        onClick={sendAck}
                        className="rh-button-primary text-sm"
                      >
                        {t("login.modal.submit")}
                      </button>
                    </div>
                  )}
                </div>
              )}

            {phase === "complete" && (
              <p className="text-sm text-green-600 dark:text-green-400">
                {t("login.modal.complete")}
              </p>
            )}

            {phase === "failed" && (
              <p role="alert" className="rh-error">
                {errorMsg || t("login.modal.failed")}
              </p>
            )}

            {events.length > 0 && (
              <details className="text-xs text-[var(--color-fg-muted)] mt-2">
                <summary className="cursor-pointer">
                  {t("login.modal.eventLog")} ({events.length})
                </summary>
                <ul className="mt-1 max-h-40 overflow-y-auto font-mono">
                  {events.slice(-30).map((ev, i) => (
                    <li key={i} className="truncate">
                      {ev.event}: {ev.data}
                    </li>
                  ))}
                </ul>
              </details>
            )}
          </>
        )}

        <footer className="mt-auto pt-3 flex justify-end">
          <button
            type="button"
            onClick={close}
            className="rh-button-ghost text-sm"
          >
            {t("common.close")}
          </button>
        </footer>
      </div>
    </div>
  );
}
