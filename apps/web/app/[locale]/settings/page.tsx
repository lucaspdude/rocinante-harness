"use client";

// Phase 5 PR-05 — /settings opens the SettingsModal.
//
// Phase-4 PR-07 created lib/settings/SettingsModal.tsx (centered modal,
// left rail, 4 sections: General / Providers / Account / Developer)
// but the legacy 5-tab page from phase-1/phase-3 was still mounted
// here. This commit wires `/settings` to the modal instead.
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

  return (
    <>
      <TopNav />
      <SettingsModal onClose={() => router.push("/agent")} />
    </>
  );
}
