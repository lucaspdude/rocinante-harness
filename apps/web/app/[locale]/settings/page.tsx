"use client";

// Phase 5 PR-05 — /settings opens the SettingsModal.
// Phase 7 — item 03: gate the route on auth. Unauthed visitors
// are redirected to /<locale>/login?next=<original> via the
// useEffect below; authed visitors see the modal as before.
//
// `?tab=` is preserved as a legacy alias for `?section=`. The 5-tab
// `tab` ids map to the 4-section modal as follows:
//   general   -> general
//   providers -> providers
//   account   -> account
//   developer -> developer
//   devices   -> account (devices live inside the Account section).
// `?sub=` for the developer tab is preserved so deep links like
// `?tab=developer&sub=clis` keep working.

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useLocalizedPath } from "../../../lib/i18n";
import { useAuthStatus } from "../../../lib/auth/auth-status";
import { tokenStore } from "../../../lib/auth/token-store";
import { TopNav } from "../../../lib/components/TopNav";
import { SettingsModal } from "../../../lib/settings/SettingsModal";

const TAB_TO_SECTION: Record<string, string> = {
  general: "general",
  providers: "providers",
  account: "account",
  developer: "developer",
  devices: "account",
};

export default function SettingsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const lp = useLocalizedPath();
  const hasToken = tokenStore.peek();
  const { loading, status } = useAuthStatus();

  // On mount, rewrite the URL from `?tab=` / `?sub=` to the canonical
  // `?section=` form used by SettingsModal. The SettingsModal also
  // accepts no params and defaults to "general"; this is purely
  // backwards compatibility for deep links.
  useEffect(() => {
    const tab = searchParams.get("tab");
    if (!tab) return;
    const section = TAB_TO_SECTION[tab];
    if (!section) return;
    const params = new URLSearchParams();
    params.set("section", section);
    const sub = searchParams.get("sub");
    if (tab === "developer" && sub) params.set("sub", sub);
    router.replace(`/settings?${params.toString()}`);
  }, [router, searchParams]);

  // Phase 7 — item 03: redirect unauthed visitors to /login?next=…
  // Triggered from useEffect so the React render body stays pure.
  useEffect(() => {
    if (loading) return;
    if (hasToken) return;
    const next = window.location.pathname + window.location.search;
    window.location.href = `${lp("/login")}?next=${encodeURIComponent(next)}`;
  }, [loading, hasToken, lp]);

  // Loading gate: render nothing while the auth status resolves,
  // to avoid a flash of the auth_missing red box (per AC5).
  if (loading || (!hasToken && status?.auth_required)) {
    return null;
  }

  return (
    <>
      <TopNav />
      <SettingsModal onClose={() => router.push("/agent")} />
    </>
  );
}
