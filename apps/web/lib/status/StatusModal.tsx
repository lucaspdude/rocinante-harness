"use client";

// StatusModal — opens from the StatusPill. Shows the current api /
// omp version, last successful + last failed check timestamps, and
// a "Re-check now" button. Click outside or press Escape to close.

import { useEffect } from "react";
import { useT } from "../i18n";

interface StatusModalProps {
  apiVersion: string;
  ompVersion: string;
  lastOkAt: number | null;
  lastFailAt: number | null;
  lastError: string | null;
  onClose: () => void;
  onRecheck: () => void;
}

function formatTimestamp(value: number | null): string {
  if (value === null) return "—";
  try {
    return new Date(value).toLocaleString();
  } catch {
    return String(value);
  }
}

export function StatusModal(props: StatusModalProps) {
  const t = useT();

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") props.onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [props]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="status-modal-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) props.onClose();
      }}
    >
      <div className="rh-card w-full max-w-md flex flex-col gap-4">
        <header className="flex items-center justify-between">
          <h2 id="status-modal-title" className="text-base font-medium">
            {t("status.modal.title")}
          </h2>
          <button
            type="button"
            onClick={props.onClose}
            aria-label={t("common.close")}
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            ×
          </button>
        </header>

        <dl className="flex flex-col gap-3 text-sm">
          <div className="flex justify-between gap-2">
            <dt className="text-[var(--color-fg-muted)]">
              {t("status.modal.apiVersion")}
            </dt>
            <dd className="font-mono">{props.apiVersion || "—"}</dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt className="text-[var(--color-fg-muted)]">
              {t("status.modal.ompVersion")}
            </dt>
            <dd className="font-mono">{props.ompVersion || "—"}</dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt className="text-[var(--color-fg-muted)]">
              {t("status.modal.lastOk")}
            </dt>
            <dd className="font-mono">{formatTimestamp(props.lastOkAt)}</dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt className="text-[var(--color-fg-muted)]">
              {t("status.modal.lastFail")}
            </dt>
            <dd className="font-mono">{formatTimestamp(props.lastFailAt)}</dd>
          </div>
          {props.lastError && (
            <div className="text-xs text-[var(--color-danger)]">
              {props.lastError}
            </div>
          )}
        </dl>

        <footer className="flex justify-end gap-2">
          <button
            type="button"
            className="rh-button-ghost"
            onClick={props.onClose}
          >
            {t("common.close")}
          </button>
          <button
            type="button"
            className="rh-button-primary"
            onClick={props.onRecheck}
          >
            {t("status.modal.recheck")}
          </button>
        </footer>
      </div>
    </div>
  );
}
