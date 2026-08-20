"use client";

// Settings page (PR-07) — renders the centered `SettingsModal` over a
// dimmed backdrop. The 4-/5-tab page that lived here from phase-1/3
// collapsed into the modal's left-rail navigation; this file exists
// now only to host the route (`/settings`) so existing TopNav links
// and the legacy `?tab=` deep links still resolve to the new modal.
//
// Deep links:
//   - `/settings`            → opens on the last-used section
//                              (or "general" on a fresh device)
//   - `/settings?section=X`  → opens on section X (general / providers
//                              / account / developer)
//
// Older links from phase-1 (`/settings?tab=X&sub=Y`) still work: the
// modal ignores them but doesn't crash; the page renders the modal
// in its default state.

import { useRouter } from "next/navigation";
import { useI18n } from "../../../lib/i18n";
import { SettingsModal } from "../../../lib/settings/SettingsModal";

export default function SettingsPage() {
  const router = useRouter();
  const i18n = useI18n();

  return (
    <SettingsModal
      onClose={() => {
        // The modal is the entire page experience. Close = route
        // back to the locale's home (chat list). Using replace
        // rather than back() because the user may have deep-linked
        // straight to /settings from outside the app.
        const home = i18n.locale === "en-US" ? "/" : `/${i18n.locale}`;
        router.replace(home);
      }}
    />
  );
}
