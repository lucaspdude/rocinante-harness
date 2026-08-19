import { I18nProvider } from "../../lib/i18n";
import type { Locale } from "../../lib/i18n/schema";
import { ToastProvider, ToastViewport } from "../../lib/toast";
import { StatusMount } from "../../lib/status/StatusMount";

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
      </ToastProvider>
    </I18nProvider>
  );
}
