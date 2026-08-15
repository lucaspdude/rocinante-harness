import { z } from "zod";

export const DictionarySchema = z.record(z.string(), z.string());
export type Dictionary = z.infer<typeof DictionarySchema>;

export const SUPPORTED_LOCALES = ["en-US", "pt-BR"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "en-US";

export function isLocale(value: string): value is Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(value);
}
