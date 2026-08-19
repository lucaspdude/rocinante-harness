// URL-driven state for the Settings page tabs.
//
// Reading: the active tab is sourced from the URL `?tab=` query
// string. When the URL has no `?tab=`, the hook falls back to the
// legacy `rh:active-settings-tab` localStorage value (set by the
// phase-1 PR-04 settings page). When neither is present, the tab
// defaults to `"general"`. Unknown tab/sub values also fall back
// to the default.
//
// Writing: `setTab(tab, sub?)` updates the URL via
// `router.replace(...)` (no history entry — back button skips URL
// changes) and persists the tab to localStorage so a deep link
// without `?tab=` still lands on the right view.
//
// `isReady` mirrors the `router.isReady` semantics from the Pages
// Router: it is `false` until the first render that has access to
// the search params, and `true` afterwards. Callers should gate
// scroll-into-view and other DOM work on `isReady` to avoid
// racing the initial hydration.

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useLocalizedPath } from "../i18n";

export const TAB_IDS = [
  "general",
  "providers",
  "account",
  "developer",
  "devices",
] as const;
export type TabId = (typeof TAB_IDS)[number];

export const SUB_IDS = ["ssh", "servers", "clis"] as const;
export type SubId = (typeof SUB_IDS)[number];

const STORAGE_KEY = "rh:active-settings-tab";

const DEFAULT_TAB: TabId = "general";
const DEFAULT_SUB: SubId = "ssh";

// Type guards — preserve narrowing in callers.
function isTabId(v: string | null): v is TabId {
  return v !== null && (TAB_IDS as readonly string[]).includes(v);
}

function isSubId(v: string | null): v is SubId {
  return v !== null && (SUB_IDS as readonly string[]).includes(v);
}

export interface ActiveTabState {
  tab: TabId;
  sub: SubId;
  setTab: (tab: TabId, sub?: SubId) => void;
  isReady: boolean;
}

export function useActiveTab(): ActiveTabState {
  const router = useRouter();
  const lp = useLocalizedPath();
  const searchParams = useSearchParams();

  // `useSearchParams` returns a stable `ReadonlyURLSearchParams`
  // handle in the App Router. On the very first render the value
  // is already populated; we still expose `isReady` so the page
  // can gate the scroll-into-view effect (matches the pages-router
  // `router.isReady` contract referenced by PR-10 spec §5.2).
  const [isReady, setIsReady] = useState(false);
  useEffect(() => {
    setIsReady(true);
  }, []);

  const queryTab = searchParams?.get("tab") ?? null;
  const querySub = searchParams?.get("sub") ?? null;

  const tab: TabId = useMemo(() => {
    if (isTabId(queryTab)) return queryTab;
    if (!isReady) return DEFAULT_TAB;
    if (queryTab === null && typeof window !== "undefined") {
      // URL has no `?tab` — fall back to localStorage.
      const stored = window.localStorage.getItem(STORAGE_KEY);
      if (isTabId(stored)) return stored;
    }
    // Unknown tab value (or no storage access) — fall back to default.
    return DEFAULT_TAB;
  }, [queryTab, isReady]);

  const sub: SubId = useMemo(() => {
    if (isSubId(querySub)) return querySub;
    return DEFAULT_SUB;
  }, [querySub]);

  const setTab = useCallback(
    (nextTab: TabId, nextSub?: SubId) => {
      const params = new URLSearchParams();
      params.set("tab", nextTab);
      // Only persist `sub` for the developer tab; other tabs have
      // no sub-section, so the query stays clean.
      const subValue: SubId | undefined =
        nextTab === "developer" ? nextSub ?? DEFAULT_SUB : undefined;
      if (subValue) params.set("sub", subValue);
      const qs = params.toString();
      router.replace(lp(`/settings${qs ? `?${qs}` : ""}`));
      if (typeof window !== "undefined") {
        window.localStorage.setItem(STORAGE_KEY, nextTab);
      }
    },
    [router, lp],
  );

  return { tab, sub, setTab, isReady };
}
