"use client";

// useSelectedModel — manages the user's last-picked model id
// (PR-02). Persists to localStorage so reloads preserve the choice
// and the picker opens pre-selected. When localStorage is empty,
// falls back to `defaultModel` from /api/v1/meta (passed in by the
// caller — see useDefaultModel).
//
// The hook intentionally does NOT call setState on every defaultModel
// change: once the user has picked a model, subsequent changes to
// the server default shouldn't clobber the user's selection. Only
// the initial empty seed consults defaultModel.

import { useCallback, useEffect, useState } from "react";

const SELECTED_KEY = "rh:selected-model";

function readStored(): string {
  if (typeof window === "undefined") return "";
  try {
    return window.localStorage.getItem(SELECTED_KEY) ?? "";
  } catch {
    return "";
  }
}

function writeStored(id: string): void {
  if (typeof window === "undefined") return;
  try {
    if (id) {
      window.localStorage.setItem(SELECTED_KEY, id);
    } else {
      window.localStorage.removeItem(SELECTED_KEY);
    }
  } catch {
    // localStorage may be disabled (private mode, quota); fall
    // through silently — the in-memory selection still works for
    // the current session.
  }
}

export function useSelectedModel(defaultModel = ""): {
  selectedModel: string;
  selectModel: (id: string) => void;
  clearModel: () => void;
} {
  const [selectedModel, setSelectedModel] = useState<string>("");
  const [hydrated, setHydrated] = useState<boolean>(false);

  // Hydrate from localStorage on mount. We deliberately only do
  // this once; subsequent edits flow through selectModel.
  useEffect(() => {
    const stored = readStored();
    if (stored) {
      setSelectedModel(stored);
    } else if (defaultModel) {
      // Fall back to the server default when nothing is stored yet.
      setSelectedModel(defaultModel);
      writeStored(defaultModel);
    }
    setHydrated(true);
    // defaultModel is intentionally NOT a dep: we only seed the
    // initial selection from it, then the user's stored value wins.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const selectModel = useCallback((id: string) => {
    setSelectedModel(id);
    writeStored(id);
  }, []);

  const clearModel = useCallback(() => {
    setSelectedModel("");
    writeStored("");
  }, []);

  // Suppress unused-var lint for hydrated; the flag is exposed for
  // future consumers that want to wait for hydration before
  // rendering the picker trigger.
  void hydrated;

  return { selectedModel, selectModel, clearModel };
}