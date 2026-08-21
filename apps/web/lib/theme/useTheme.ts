"use client";

import { useCallback, useEffect, useState } from "react";

// Theme preference values: "light" / "dark" forced, or "system" follows
// the OS `prefers-color-scheme` media query.
//
// Persistence key: localStorage["rh-theme"].
// DOM hooks:
//   - document.documentElement.dataset.theme   ("dark" | "light" | "")
//   - document.documentElement.style.colorScheme ("dark" | "light" | "")
//
// The `data-theme` attribute is the legacy hook used by SafeMarkdown
// (lib/agent/SafeMarkdown.tsx) and FileEditor (lib/files/FileEditor.tsx)
// via MutationObserver. Keep the attribute name stable.
// The `color-scheme` style is the modern hook driving native form
// controls, scrollbars, and `<input type="date">` palettes.

export type ThemePreference = "light" | "dark" | "system";
export type ResolvedTheme = "light" | "dark";

const STORAGE_KEY = "rh-theme";

// Legacy `OMP_WEB_THEME` shim values map to the modern preference
// space. Used by older install scripts and any stray env-driven
// theme flags. See PR-08 spec §Risks.
const LEGACY_THEME_MAP: Record<string, ThemePreference> = {
  "warm-paper": "light",
  "warm-ember": "dark",
  light: "light",
  dark: "dark",
  system: "system",
};

function readStoredPreference(): ThemePreference {
  if (typeof window === "undefined") return "system";
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (raw && (raw === "light" || raw === "dark" || raw === "system")) {
      return raw;
    }
    // Legacy shim — accept any historical env-derived value once.
    const legacy = window.localStorage.getItem("omp-web-theme");
    if (legacy && legacy in LEGACY_THEME_MAP) {
      return LEGACY_THEME_MAP[legacy] ?? "system";
    }
  } catch {
    // localStorage may be blocked (private mode, sandbox). Fall through.
  }
  return "system";
}



function applyToDocument(pref: ThemePreference) {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  // data-theme: "" for "system" (lets CSS fall back to OS), else the value.
  root.dataset.theme = pref === "system" ? "" : pref;
  // color-scheme style attribute drives native form/scrollbar palettes.
  // Empty for "system" so the UA picks from prefers-color-scheme.
  if (pref === "system") {
    root.style.colorScheme = "";
  } else {
    root.style.colorScheme = pref;
  }
}

export interface UseThemeApi {
  preference: ThemePreference;
  resolved: ResolvedTheme;
  setPreference: (next: ThemePreference) => void;
}

export function useTheme(): UseThemeApi {
  const [preference, setPreferenceState] = useState<ThemePreference>("system");
  const [systemDark, setSystemDark] = useState<boolean>(false);

  // Initial mount: read localStorage + subscribe to media query.
  useEffect(() => {
    const initial = readStoredPreference();
    setPreferenceState(initial);
    applyToDocument(initial);
    setSystemDark(window.matchMedia("(prefers-color-scheme: dark)").matches);

    if (typeof window === "undefined") return;
    const mql = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = (e: MediaQueryListEvent) => setSystemDark(e.matches);
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, []);

  const setPreference = useCallback((next: ThemePreference) => {
    setPreferenceState(next);
    if (typeof window !== "undefined") {
      try {
        window.localStorage.setItem(STORAGE_KEY, next);
      } catch {
        // ignore — quota / private mode
      }
    }
    applyToDocument(next);
  }, []);

  const resolved: ResolvedTheme =
    preference === "system" ? (systemDark ? "dark" : "light") : preference;

  return { preference, resolved, setPreference };
}

// Inline script string used by app/layout.tsx <Script> block to
// prevent a flash of the wrong theme on first paint (FOUC).
// Runs synchronously before React hydrates.
//
// IMPORTANT: this must remain a pure DOM/localStorage operation — no
// React state, no module imports. Keep it tiny.
export const THEME_BOOTSTRAP_SCRIPT = `(() => {
  try {
    var KEY = "rh-theme";
    var raw = localStorage.getItem(KEY);
    var pref = (raw === "light" || raw === "dark" || raw === "system") ? raw : "system";
    var root = document.documentElement;
    root.dataset.theme = pref === "system" ? "" : pref;
    if (pref !== "system") root.style.colorScheme = pref;
  } catch (e) {}
})();`;