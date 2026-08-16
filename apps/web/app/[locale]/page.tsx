"use client";

import { useT } from "../../lib/i18n";

export default function HomePage() {
  const t = useT();
  return (
    <main className="max-w-2xl mx-auto px-4 py-16">
      <h1 className="text-4xl font-semibold mb-3">{t("app.name")}</h1>
      <p className="text-[var(--color-fg-muted)] text-lg mb-8">
        {t("app.tagline")}
      </p>
      <div className="rh-card">
        <p className="text-[var(--color-fg-muted)]">
          {t("agent.empty")}
        </p>
        <div className="mt-4 flex gap-3">
          <a
            href="login"
            className="rh-button-primary inline-block"
          >
            {t("login.submit")}
          </a>
          <a
            href="settings"
            className="rh-button-ghost inline-block"
          >
            {t("settings.title")}
          </a>
        </div>
      </div>
    </main>
  );
}
