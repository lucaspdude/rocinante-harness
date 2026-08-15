import type { Locale } from "./schema";
import { DEFAULT_LOCALE } from "./schema";

/**
 * Negotiate a locale from the Accept-Language header.
 * Rules: missing or "*" → default; pt-BR wins when present;
 *   any other language → default.
 */
export function negotiateLocale(acceptLanguage: string | null | undefined): Locale {
  if (!acceptLanguage || acceptLanguage.trim() === "" || acceptLanguage === "*") {
    return DEFAULT_LOCALE;
  }
  const entries = acceptLanguage.split(",").map((part) => {
    const trimmed = part.trim();
    const semi = trimmed.indexOf(";");
    const lang = (semi === -1 ? trimmed : trimmed.substring(0, semi)).trim().toLowerCase();
    const attrs = semi === -1 ? "" : trimmed.substring(semi + 1);
    const qAttr = attrs.split(";").find((a) => a.trim().startsWith("q=")) ?? "";
    const q = qAttr ? parseFloat(qAttr.split("=")[1] ?? "1") : 1.0;
    return { lang, q: isNaN(q) ? 1.0 : q };
  });
  entries.sort((a, b) => b.q - a.q);
  for (const { lang } of entries) {
    if (lang === "pt-br" || lang.startsWith("pt-")) return "pt-BR";
    if (lang === "en-us" || lang.startsWith("en-")) return "en-US";
    if (lang === "en") return "en-US";
    if (lang === "pt") return "pt-BR";
  }
  return DEFAULT_LOCALE;
}
