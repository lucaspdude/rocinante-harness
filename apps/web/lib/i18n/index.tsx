"use client";

import { createContext, useContext, useEffect, useState } from "react";
import type { Locale } from "./schema";
import { DEFAULT_LOCALE } from "./schema";
import enUS from "./en-US.json";
import ptBR from "./pt-BR.json";
import { DictionarySchema, type Dictionary } from "./schema";

const dictionaries: Record<Locale, Dictionary> = {
  "en-US": DictionarySchema.parse(enUS),
  "pt-BR": DictionarySchema.parse(ptBR),
};

interface I18nContext {
  locale: Locale;
  t: (key: string, vars?: Record<string, string | number>) => string;
  setLocale: (next: Locale) => void;
  localizedPath: (path: string) => string;
}

const I18nContextImpl = createContext<I18nContext | null>(null);

export function I18nProvider({
  initialLocale,
  children,
}: {
  initialLocale: Locale;
  children: React.ReactNode;
}) {
  const [locale, setLocaleState] = useState<Locale>(initialLocale);

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  function setLocale(next: Locale) {
    setLocaleState(next);
    document.cookie = `rh-locale=${next}; path=/; max-age=31536000; samesite=lax`;
  }

  function localizedPath(path: string): string {
    if (!path.startsWith("/")) path = "/" + path;
    return `/${locale}${path === "/" ? "" : path}`;
  }

  function t(key: string, vars?: Record<string, string | number>) {
    const dict = dictionaries[locale] ?? dictionaries[DEFAULT_LOCALE];
    let value = dict[key];
    if (value === undefined) {
      value = dictionaries[DEFAULT_LOCALE][key] ?? key;
    }
    if (vars) {
      for (const [k, v] of Object.entries(vars)) {
        value = value.replace(new RegExp(`\\{${k}\\}`, "g"), String(v));
      }
    }
    return value;
  }

  return (
    <I18nContextImpl.Provider value={{ locale, t, setLocale, localizedPath }}>
      {children}
    </I18nContextImpl.Provider>
  );
}

export function useI18n(): I18nContext {
  const ctx = useContext(I18nContextImpl);
  if (ctx === null) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return ctx;
}

export function useLocale(): Locale {
  return useI18n().locale;
}

// Returns a function that turns a path relative to the locale
// (e.g. "/login") into an absolute path with the locale prefix
// (e.g. "/en-US/login"). Use this instead of hard-coding
// `/${locale}` in JSX so a future locale switch just works.
export function useLocalizedPath(): (path: string) => string {
  return useI18n().localizedPath;
}

export function useT() {
  return useI18n().t;
}
