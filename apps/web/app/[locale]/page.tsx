"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useT, useLocalizedPath } from "../../lib/i18n";
import { tokenStore } from "../../lib/auth/token-store";
import { useAuthStatus } from "../../lib/auth/auth-status";
import { TopNav } from "../../lib/components/TopNav";
import { api } from "../../lib/api/client";

export default function HomePage() {
  const t = useT();
  const router = useRouter();
  const lp = useLocalizedPath();
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null);
  const { loading: authLoading, status: authStatus } = useAuthStatus();

  // Fetch api onboarding state.
  useEffect(() => {
    api
      .get<{ initialized: boolean }>("/api/v1/onboarding/status")
      .then((s) => setNeedsSetup(!s.initialized))
      .catch(() => setNeedsSetup(false));
  }, []);

  // Forward authed user to /agent/new (the daily-driver surface).
  // Runs after needsSetup resolves. Replace (not push) so the back
  // button skips the marketing page. Onboarding params suppress
  // the redirect so the user can recover mid-flow.
  useEffect(() => {
    if (needsSetup !== false) return;
    if (typeof window === "undefined") return;
    const sp = new URLSearchParams(window.location.search);
    if (
      sp.has("onboarding") ||
      sp.has("code") ||
      sp.has("device") ||
      sp.has("token")
    ) {
      return;
    }
    if (tokenStore.peek()) {
      router.replace(lp("/agent/new"));
    }
  }, [needsSetup, router, lp]);

  // Phase 7 — item 01: redirect returning users (cookie present
  // + no token) to /login. Brand-new visitors (no cookie) still
  // see the "Sign in" CTA. Triggered from useEffect so the React
  // render body stays pure.
  useEffect(() => {
    if (needsSetup !== false) return;
    if (typeof window === "undefined") return;
    if (authLoading) return;
    if (!authStatus) return;
    if (tokenStore.peek()) return; // already handled by the authed branch
    if (!authStatus.device_known) return; // first visit — show CTA
    const next = window.location.pathname + window.location.search;
    window.location.href = `${lp("/login")}?next=${encodeURIComponent(next)}`;
  }, [needsSetup, authLoading, authStatus, lp]);

  // Phase 7.5 item A: first-visit visitors (no cookie, no token)
  // see the "Sign in" CTA render first, then auto-redirect after
  // a short delay so the page does not feel like a marketing
  // stop. Returning users (cookie present) are handled by the
  // useEffect above with no visible CTA. The 600 ms delay is
  // short enough to feel snappy on a LAN install but long
  // enough to paint the CTA once (so users coming from a
  // search engine result see the value-prop before the URL
  // changes).
  useEffect(() => {
    if (needsSetup !== false) return;
    if (typeof window === "undefined") return;
    if (authLoading) return;
    if (!authStatus) return;
    if (tokenStore.peek()) return; // authed — already handled
    if (authStatus.device_known) return; // returning — useEffect above
    const t = window.setTimeout(() => {
      const next = window.location.pathname + window.location.search;
      window.location.href = `${lp("/login")}?next=${encodeURIComponent(next)}`;
    }, 600);
    return () => window.clearTimeout(t);
  }, [needsSetup, authLoading, authStatus, lp]);

  // Phase 7 — item 01 AC6: token present but api re-initialised
  // (tokenStore.peek() && needsSetup === true). The token is
  // stale; the onboarding init flow clears it. Redirect to
  // /onboarding so the user can re-init the api.
  useEffect(() => {
    if (needsSetup !== true) return;
    if (typeof window === "undefined") return;
    if (!tokenStore.peek()) return;
    window.location.href = lp("/onboarding");
  }, [needsSetup, lp]);

  // While we don't yet know the api's state, render a small
  // loading hint instead of the action buttons. Avoids a
  // confusing flash of "Sign in" on a fresh install.
  if (needsSetup === null) {
    return (
      <>
        <TopNav />
        <main className="max-w-2xl mx-auto px-4 py-16">
          <p className="text-[var(--color-fg-muted)]">{t("common.loading")}</p>
        </main>
      </>
    );
  }

  if (needsSetup) {
    return (
      <>
        <TopNav />
        <main className="max-w-2xl mx-auto px-4 py-16">
          <h1 className="text-4xl font-semibold mb-3">{t("app.name")}</h1>
          <p className="text-[var(--color-fg-muted)] text-lg mb-8">
            {t("app.tagline")}
          </p>
          <div className="rh-card">
            <p className="text-[var(--color-fg-muted)] mb-4">
              {t("onboarding.subtitle")}
            </p>
            <a
              href={lp("/onboarding")}
              className="rh-button-primary inline-block"
            >
              {t("onboarding.submit")}
            </a>
          </div>
        </main>
      </>
    );
  }

  // Un-authed first-visit: single Sign in CTA. The cookie-gated
  // useEffect above already redirected returning users to
  // /login, so reaching this branch means device_known=false.
  return (
    <>
      <TopNav />
      <main className="max-w-2xl mx-auto px-4 py-16">
        <h1 className="text-4xl font-semibold mb-3">{t("app.name")}</h1>
        <p className="text-[var(--color-fg-muted)] text-lg mb-8">
          {t("app.tagline")}
        </p>
        <div className="rh-card">
          <p className="text-[var(--color-fg-muted)]">{t("agent.empty")}</p>
          <div className="mt-4 flex gap-3">
            <a
              href={lp("/login")}
              className="rh-button-primary inline-block"
            >
              {t("login.submit")}
            </a>
          </div>
        </div>
      </main>
    </>
  );
}
