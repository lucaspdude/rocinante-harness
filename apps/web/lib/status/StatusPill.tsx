"use client";

// StatusPill — small fixed bottom-right pill that visualizes the
// harness status. Click to open the StatusModal. The pill sits
// to the right of the ToastViewport (which is shifted left by
// mr-32 to leave room).

import { useState } from "react";
import { useT } from "../i18n";
import type { StatusKind } from "./useStatus";
import { StatusModal } from "./StatusModal";

interface StatusPillProps {
  kind: StatusKind;
  apiVersion: string;
  recheck: () => void;
  lastOkAt: number | null;
  lastFailAt: number | null;
  lastError: string | null;
  ompVersion: string;
}

const DOT_COLOR: Record<StatusKind, string> = {
  ok: "bg-emerald-500",
  partial: "bg-amber-500",
  fail: "bg-red-500",
  loading: "bg-zinc-500",
};

const PILL_LABEL_KEY: Record<StatusKind, string> = {
  ok: "status.pill.ok",
  partial: "status.pill.partial",
  fail: "status.pill.fail",
  loading: "status.pill.loading",
};

export function StatusPill(props: StatusPillProps) {
  const t = useT();
  const [open, setOpen] = useState(false);

  const label =
    props.kind === "ok"
      ? t("status.pill.ok", { apiVersion: props.apiVersion || "?" })
      : props.kind === "partial"
        ? t("status.pill.partial", { apiVersion: props.apiVersion || "?" })
        : t(PILL_LABEL_KEY[props.kind]);

  return (
    <>
      <button
        type="button"
        data-testid="status-pill"
        onClick={() => setOpen(true)}
        className="fixed bottom-4 right-4 z-40 flex items-center gap-2 rounded-full border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-1 text-xs shadow"
        aria-label={t("status.modal.title")}
      >
        <span
          className={`inline-block h-2 w-2 rounded-full ${DOT_COLOR[props.kind]}`}
          aria-hidden="true"
        />
        <span className="text-[var(--color-fg)]">{label}</span>
      </button>
      {open && (
        <StatusModal
          apiVersion={props.apiVersion}
          ompVersion={props.ompVersion}
          lastOkAt={props.lastOkAt}
          lastFailAt={props.lastFailAt}
          lastError={props.lastError}
          onClose={() => setOpen(false)}
          onRecheck={() => {
            props.recheck();
          }}
        />
      )}
    </>
  );
}
