"use client";

// Top navigation bar, rendered above the page content on every
// authenticated locale page. Two rows:
//
//   1. brand (left) + current locale badge (right) + Settings link
//   2. (slot left for future breadcrumbs, e.g. in the chat
//      page header)
//
// The nav intentionally lives in the locale layout (not the
// root layout) so the auth and onboarding pages — which sit
// OUTSIDE the locale segment — can opt out by not including
// it. The chat page wraps the layout in its own flex row that
// also includes the Sidebar.

import Link from "next/link";
import { useT, useI18n, useLocalizedPath } from "../i18n";
import { SUPPORTED_LOCALES, type Locale } from "../i18n/schema";

export function TopNav() {
  const t = useT();
  const i18n = useI18n();
  const lp = useLocalizedPath();
  return (
    <header className="border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
      <div className="px-4 py-2 flex items-center justify-between gap-3">
        <Link
          href={lp("/")}
          className="font-semibold text-sm hover:opacity-80 transition-opacity"
        >
          {t("app.name")}
        </Link>
        <div className="flex items-center gap-3">
          <select
            aria-label={t("settings.locale")}
            value={i18n.locale}
            onChange={(e) => i18n.setLocale(e.target.value as Locale)}
            className="text-xs bg-transparent border border-[var(--color-border)] rounded px-1.5 py-0.5"
          >
            {SUPPORTED_LOCALES.map((l) => (
              <option key={l} value={l}>
                {l}
              </option>
            ))}
          </select>
          <Link
            href={lp("/settings")}
            className="text-xs text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            {t("settings.title")}
          </Link>
        </div>
      </div>
    </header>
  );
}
