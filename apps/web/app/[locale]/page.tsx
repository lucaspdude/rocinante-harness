"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useT, useLocalizedPath } from "../../lib/i18n";
import { tokenStore } from "../../lib/auth/token-store";
import { TopNav } from "../../lib/components/TopNav";
import { api } from "../../lib/api/client";

export default function HomePage() {
  const t = useT();
  const router = useRouter();
  const lp = useLocalizedPath();
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null);

  useEffect(() => {
    api
      .get<{ initialized: boolean }>("/api/v1/onboarding/status")
      .then((s) => setNeedsSetup(!s.initialized))
      .catch(() => setNeedsSetup(false));
  }, []);

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

  // Onboarding query params (?onboarding=, ?code=, ?device=,
  // ?token=) indicate the user is mid-onboarding. Do not
  // redirect — let them land on the homepage CTAs so they can
  // recover.
  const hasOnboardingParam = (() => {
    if (typeof window === "undefined") return false;
    const sp = new URLSearchParams(window.location.search);
    return (
      sp.has("onboarding") ||
      sp.has("code") ||
      sp.has("device") ||
      sp.has("token")
    );
  })();

  // Authed user with no onboarding flow in flight: forward to
  // the daily-driver surface. The needsSetup === null branch
  // already returned a loading hint, so by the time this effect
  // fires we have a final answer. replace (not push) so the back
  // button skips the marketing page.
  useEffect(() => {
    if (needsSetup !== false) return;
    if (hasOnboardingParam) return;
    if (tokenStore.peek()) {
      router.replace(lp("/agent/new"));
    }
  }, [needsSetup, router, lp, hasOnboardingParam]);

  // Un-authed: single Sign in CTA. The old two-button gate was
  // visible-but-useless for users without a token.
  return (
    <>
      <TopNav />
      <main className="max-w-2xl mx-auto px-4 py-16">
        <h1 className="text-4xl font-semibold mb-3">{t("app.name")}</h1>
        <p className="text-[var(--color-fg-muted)] text-lg mb-8">
          {t("app.tagline")}
        </p>
        <div className="rh-card">
          <a
            href={lp("/login")}
            className="rh-button-primary inline-block"
          >
            {t("login.submit")}
          </a>
        </div>
      </main>
    </>
  );
}
