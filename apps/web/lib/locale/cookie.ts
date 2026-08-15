import type { Locale } from "../i18n/schema";
import { DEFAULT_LOCALE, isLocale } from "../i18n/schema";

export const LOCALE_COOKIE = "rh-locale";

export function readLocaleFromCookie(cookieHeader: string | null | undefined): Locale {
  if (!cookieHeader) return DEFAULT_LOCALE;
  const parts = cookieHeader.split(";");
  for (const part of parts) {
    const [name, ...rest] = part.trim().split("=");
    if (name === LOCALE_COOKIE) {
      const value = rest.join("=").trim();
      if (isLocale(value)) return value;
    }
  }
  return DEFAULT_LOCALE;
}
