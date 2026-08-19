// Pure toast-store helpers. Kept dependency-free so they can be
// unit-tested without React. The React Context provider in
// ToastProvider.tsx wraps these with useReducer.

export type ToastKind = "success" | "error" | "info" | "warning";

export interface Toast {
  id: string;
  kind: ToastKind;
  title: string;
  description?: string;
  durationMs: number;
  code?: string;
}

export const DEFAULT_DURATIONS_MS: Record<ToastKind, number> = {
  success: 5000,
  info: 5000,
  error: 8000,
  warning: 12000,
};

export const TOAST_VISIBLE_MAX = 5;

// Sequential-ish id: timestamp + monotonic counter so two toasts
// in the same millisecond still get distinct ids.
let toastIdCounter = 0;
export function nextToastId(): string {
  toastIdCounter += 1;
  return `t_${Date.now()}_${toastIdCounter}`;
}

export interface AddInput {
  kind: ToastKind;
  title: string;
  description?: string;
  durationMs?: number;
  code?: string;
}

// Add a new toast to the end of the list. Drops the oldest
// entry when the list would exceed TOAST_VISIBLE_MAX.
export function addToast(list: Toast[], input: AddInput): Toast[] {
  const id = nextToastId();
  const next: Toast = {
    id,
    kind: input.kind,
    title: input.title,
    durationMs: input.durationMs ?? DEFAULT_DURATIONS_MS[input.kind],
  };
  if (input.description !== undefined) next.description = input.description;
  if (input.code !== undefined) next.code = input.code;
  const appended = [...list, next];
  if (appended.length <= TOAST_VISIBLE_MAX) return appended;
  return appended.slice(appended.length - TOAST_VISIBLE_MAX);
}

export function dismissToast(list: Toast[], id: string): Toast[] {
  return list.filter((t) => t.id !== id);
}

export function clearToasts(): Toast[] {
  return [];
}

export interface ExtractedError {
  code?: string;
  message: string;
}

function readBodyCode(value: unknown): string | undefined {
  if (!value || typeof value !== "object") return undefined;
  const obj = value as Record<string, unknown>;
  if ("code" in obj && typeof obj.code === "string") return obj.code;
  return undefined;
}

function readBodyMessage(value: unknown): string | undefined {
  if (!value || typeof value !== "object") return undefined;
  const obj = value as Record<string, unknown>;
  if (typeof obj.message === "string") return obj.message;
  return undefined;
}

function readTopCode(value: Record<string, unknown>): string | undefined {
  if (typeof value.code === "string") return value.code;
  return undefined;
}

function readTopMessage(value: Record<string, unknown>): string | undefined {
  if (typeof value.message === "string") return value.message;
  return undefined;
}

// Pull a structured api-style error into a {code, message} pair.
// Accepts:
//   - an Error (no code; message = err.message; honors .body for ApiClientError)
//   - an object with { body: { code, message } } or { code, message }
//   - a string (treated as message)
//   - null/undefined → empty pair (caller decides whether to no-op)
export function extractError(err: unknown): ExtractedError {
  if (err === null || err === undefined) return { message: "" };
  if (typeof err === "string") return { message: err };
  if (err instanceof Error) {
    const body = (err as Error & { body?: unknown }).body;
    const code = readBodyCode(body);
    const message = readBodyMessage(body) ?? err.message;
    return code ? { code, message } : { message };
  }
  if (typeof err === "object") {
    const obj = err as Record<string, unknown>;
    const body = obj.body;
    if (body && typeof body === "object") {
      const code = readBodyCode(body);
      const message = readBodyMessage(body) ?? readTopMessage(obj) ?? "";
      return code ? { code, message } : { message };
    }
    const code = readTopCode(obj);
    const message = readTopMessage(obj) ?? "";
    return code ? { code, message } : { message };
  }
  return { message: String(err) };
}
