"use client";

import { useEffect, useState } from "react";
import { useT, useLocalizedPath } from "../../lib/i18n";
import { tokenStore } from "../../lib/auth/token-store";
import { TopNav } from "../../lib/components/TopNav";
import { api } from "../../lib/api/client";

export default function HomePage() {
  const t = useT();
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
              href={tokenStore.peek() ? lp("/agent/new") : lp("/login")}
              className="rh-button-primary inline-block"
            >
              {tokenStore.peek() ? t("agent.newSession") : t("login.submit")}
            </a>
            <a
              href={lp("/settings")}
              className="rh-button-ghost inline-block"
            >
              {t("settings.title")}
            </a>
          </div>
        </div>
      </main>
    </>
  );
}
