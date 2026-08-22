// localStorage key + SSR-safe accessors for the user's last
// selected model id (PR-3). The composer reads `rh:selected-model`
// on mount and writes it on every model pick so the picker
// re-opens on the same model after a refresh.
//
// Imported from a Server Component is a no-op (no `window`).

export const SELECTED_MODEL_STORAGE_KEY = "rh:selected-model";

export function readSelectedModelId(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(SELECTED_MODEL_STORAGE_KEY);
  } catch {
    return null;
  }
}

export function writeSelectedModelId(id: string): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(SELECTED_MODEL_STORAGE_KEY, id);
  } catch {
    // Quota / disabled storage — silently ignore. The next pick
    // retries; the user only loses persistence, not the pick.
  }
}
