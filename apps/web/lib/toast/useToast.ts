"use client";

// useToast — hook that returns the convenience methods
// (show / success / error / info / warning / dismiss). The
// "error" variant extracts { code, message } from the supplied
// error using the pure extractError helper, then routes the
// extracted code into the toast's badge slot.

import { useCallback } from "react";
import { extractError, type ToastKind } from "./store";
import { useToastContextInternal } from "./ToastProvider";

export interface ToastApi {
  show: (input: {
    kind: ToastKind;
    title: string;
    description?: string;
    durationMs?: number;
    code?: string;
  }) => string;
  success: (title: string, description?: string) => string;
  error: (errOrTitle: unknown, description?: string) => string;
  info: (title: string, description?: string) => string;
  warning: (title: string, description?: string) => string;
  dismiss: (id: string) => void;
}

export function useToast(): ToastApi {
  const ctx = useToastContextInternal();
  const show = ctx.show;
  const dismiss = ctx.dismiss;

  const success = useCallback(
    (title: string, description?: string) =>
      show({ kind: "success", title, ...(description !== undefined && { description }) }),
    [show],
  );

  const error = useCallback(
    (errOrTitle: unknown, description?: string) => {
      if (typeof errOrTitle === "string") {
        return show({ kind: "error", title: errOrTitle, ...(description !== undefined && { description }) });
      }
      const { code, message } = extractError(errOrTitle);
      if (!message && !code) {
        return show({ kind: "error", title: "Error" });
      }
      return show({
        kind: "error",
        title: message || "Error",
        ...(code ? { code } : {}),
      });
    },
    [show],
  );

  const info = useCallback(
    (title: string, description?: string) =>
      show({ kind: "info", title, ...(description !== undefined && { description }) }),
    [show],
  );

  const warning = useCallback(
    (title: string, description?: string) =>
      show({ kind: "warning", title, ...(description !== undefined && { description }) }),
    [show],
  );

  return { show, success, error, info, warning, dismiss };
}
