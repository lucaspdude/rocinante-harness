"use client";

// ToastViewport — fixed bottom-right container that renders the
// live toast list. Each toast:
//   - uses the rh-card component class
//   - shows a 4px left border colored by kind
//   - has a [code] badge top-right when a code was provided
//   - has a close (×) button wired to t('toast.dismiss')
//   - is click-to-dismiss and hover-pause-able
//
// Hover-pause is implemented via a per-toast useEffect that
// captures the auto-dismiss timer id and pauses / resumes it
// based on mouseenter / mouseleave.

import { useEffect, useRef } from "react";
import { useT } from "../i18n";
import type { Toast } from "./store";
import { useToastContextInternal } from "./ToastProvider";

const KIND_LABEL_KEY: Record<Toast["kind"], string> = {
  success: "toast.successBadge",
  error: "toast.errorBadge",
  info: "toast.infoBadge",
  warning: "toast.warningBadge",
};

const KIND_BORDER: Record<Toast["kind"], string> = {
  success: "border-l-emerald-500",
  error: "border-l-red-500",
  info: "border-l-blue-500",
  warning: "border-l-amber-500",
};

type ToastTimer = ReturnType<typeof setTimeout> | null;

function ToastItem({ toast }: { toast: Toast }) {
  const t = useT();
  const { dismiss } = useToastContextInternal();
  const timerRef = useRef<ToastTimer>(null);
  const remainingRef = useRef<number>(toast.durationMs);
  const startedAtRef = useRef<number>(Date.now());
  const dismissedRef = useRef<boolean>(false);

  useEffect(() => {
    if (dismissedRef.current) return;
    function arm(ms: number) {
      clearTimeout(timerRef.current ?? undefined);
      startedAtRef.current = Date.now();
      timerRef.current = setTimeout(() => {
        dismissedRef.current = true;
        dismiss(toast.id);
      }, ms);
    }
    arm(remainingRef.current);
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        const elapsed = Date.now() - startedAtRef.current;
        remainingRef.current = Math.max(0, remainingRef.current - elapsed);
      }
    };
  }, [dismiss, toast.id]);

  function onMouseEnter() {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
      const elapsed = Date.now() - startedAtRef.current;
      remainingRef.current = Math.max(0, remainingRef.current - elapsed);
    }
  }

  function onMouseLeave() {
    if (dismissedRef.current) return;
    clearTimeout(timerRef.current ?? undefined);
    startedAtRef.current = Date.now();
    timerRef.current = setTimeout(() => {
      dismissedRef.current = true;
      dismiss(toast.id);
    }, remainingRef.current);
  }

  function onClick() {
    if (dismissedRef.current) return;
    dismissedRef.current = true;
    clearTimeout(timerRef.current ?? undefined);
    dismiss(toast.id);
  }

  return (
    <div
      role="status"
      data-testid={`toast-${toast.kind}`}
      onClick={onClick}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      className={`rh-card border-l-4 ${KIND_BORDER[toast.kind]} relative cursor-pointer pr-8`}
    >
      {toast.code ? (
        <span
          aria-hidden="true"
          className="absolute top-2 right-2 text-[10px] font-mono px-1.5 py-0.5 rounded bg-[var(--color-bg-elevated)] text-[var(--color-fg-muted)]"
          title={toast.code}
        >
          [{toast.code}]
        </span>
      ) : null}
      <div className="flex flex-col gap-1">
        <span className="text-[10px] font-semibold uppercase tracking-wide text-[var(--color-fg-muted)]">
          {t(KIND_LABEL_KEY[toast.kind])}
        </span>
        <span className="text-sm text-[var(--color-fg)]">{toast.title}</span>
        {toast.description ? (
          <span className="text-xs text-[var(--color-fg-muted)]">
            {toast.description}
          </span>
        ) : null}
      </div>
      <button
        type="button"
        aria-label={t("toast.dismiss")}
        onClick={(e) => {
          e.stopPropagation();
          onClick();
        }}
        className="absolute bottom-2 right-2 text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] text-sm"
      >
        ×
      </button>
    </div>
  );
}

export function ToastViewport() {
  const { toasts } = useToastContextInternal();
  return (
    <div
      aria-live="polite"
      aria-atomic="false"
      data-testid="toast-viewport"
      className="fixed bottom-4 right-4 mr-32 z-50 flex flex-col gap-2 max-w-sm"
    >
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} />
      ))}
    </div>
  );
}
