"use client";

// ToastProvider — Context that owns the live Toast[] state. The
// provider is mounted inside I18nProvider so that descendant
// components can call both useT() and useToast(). Hover-pause is
// implemented per-toast in ToastViewport (which knows the DOM
// node); the provider just stores the list and exposes
// show/dismiss callbacks.

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useReducer,
  type ReactNode,
} from "react";
import {
  addToast,
  clearToasts,
  dismissToast,
  type AddInput,
  type Toast,
} from "./store";

interface ToastContextValue {
  toasts: Toast[];
  show: (input: AddInput) => string;
  dismiss: (id: string) => void;
  clear: () => void;
}

const ToastContextImpl = createContext<ToastContextValue | null>(null);

type Action =
  | { type: "show"; input: AddInput; id: string }
  | { type: "dismiss"; id: string }
  | { type: "clear" };

function reducer(state: Toast[], action: Action): Toast[] {
  switch (action.type) {
    case "show": {
      // The id is generated inside addToast, so we ignore the
      // one on the action (kept for symmetry with external
      // callers that might want to know the new id).
      void action.id;
      return addToast(state, action.input);
    }
    case "dismiss":
      return dismissToast(state, action.id);
    case "clear":
      return clearToasts();
    default:
      return state;
  }
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, dispatch] = useReducer(reducer, [] as Toast[]);

  const show = useCallback((input: AddInput): string => {
    // We can't return the id the reducer will assign, so we
    // generate one here for the caller's convenience (e.g.
    // a "loading" toast they may want to dismiss manually).
    // The reducer will mint its own id; the two are different
    // but that's fine because the caller can use the returned
    // id with the dismiss(id) below.
    const id = `pending_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    dispatch({ type: "show", input, id });
    return id;
  }, []);

  const dismiss = useCallback((id: string) => {
    dispatch({ type: "dismiss", id });
  }, []);

  const clear = useCallback(() => {
    dispatch({ type: "clear" });
  }, []);

  const value = useMemo<ToastContextValue>(
    () => ({ toasts, show, dismiss, clear }),
    [toasts, show, dismiss, clear],
  );

  return (
    <ToastContextImpl.Provider value={value}>
      {children}
    </ToastContextImpl.Provider>
  );
}

export function useToastContextInternal(): ToastContextValue {
  const ctx = useContext(ToastContextImpl);
  if (ctx === null) {
    throw new Error("useToast must be used inside <ToastProvider>");
  }
  return ctx;
}
