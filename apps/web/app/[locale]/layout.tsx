import { I18nProvider } from "../../lib/i18n";
import type { Locale } from "../../lib/i18n/schema";
import { ToastProvider, ToastViewport } from "../../lib/toast";
import { StatusMount } from "../../lib/status/StatusMount";
import { AuthExpiredListener } from "../../lib/auth/AuthExpiredListener";

export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  return (
    <I18nProvider initialLocale={locale as Locale}>
      <ToastProvider>
        {children}
        <ToastViewport />
        <StatusMount />
        {/* Phase 7 — item 03: client child that listens for the
            `rh:auth:expired` window event and clears the token
            + redirects to /<locale>/login?next=…. */}
        <AuthExpiredListener />
      </ToastProvider>
    </I18nProvider>
  );
}
