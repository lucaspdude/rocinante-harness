// Public re-exports for the toast infra. Importing components
// and hooks via this barrel keeps the call sites short and lets
// us reorganize internals without touching the consumers.

export { ToastProvider, useToastContextInternal } from "./ToastProvider";
export { ToastViewport } from "./ToastViewport";
export { useToast, type ToastApi } from "./useToast";
export {
  addToast,
  dismissToast,
  extractError,
  nextToastId,
  type AddInput,
  type ExtractedError,
  type Toast,
  type ToastKind,
  DEFAULT_DURATIONS_MS,
  TOAST_VISIBLE_MAX,
} from "./store";
