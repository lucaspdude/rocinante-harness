"use client";

import { Sun, Moon, Monitor } from "lucide-react";
import { useT } from "../i18n";
import { useTheme, type ThemePreference } from "./useTheme";

const OPTIONS: Array<{
  value: ThemePreference;
  icon: typeof Sun;
  i18nKey: string;
  ariaKey: string;
}> = [
  { value: "light", icon: Sun, i18nKey: "settings.themeLight", ariaKey: "theme.light" },
  { value: "dark", icon: Moon, i18nKey: "settings.themeDark", ariaKey: "theme.dark" },
  { value: "system", icon: Monitor, i18nKey: "settings.themeSystem", ariaKey: "theme.system" },
];

// Three-button segmented control bound to useTheme(). Replaces the
// legacy <select> in apps/web/app/[locale]/settings/page.tsx.
// Follows the harness reference pattern (docs/ui-ux-references/
// desktop.md §7): a row of three large icon-buttons, the active one
// filled with the brand tint.
export function ThemePicker() {
  const t = useT();
  const { preference, setPreference } = useTheme();

  return (
    <div
      role="radiogroup"
      aria-label={t("settings.theme")}
      data-testid="theme-picker"
      className="rh-theme-picker"
    >
      {OPTIONS.map(({ value, icon: Icon, i18nKey, ariaKey }) => {
        const active = preference === value;
        return (
          <button
            key={value}
            type="button"
            role="radio"
            aria-checked={active}
            aria-label={t(ariaKey)}
            data-active={active}
            onClick={() => setPreference(value)}
            className="rh-theme-picker-option"
          >
            <Icon size={20} aria-hidden />
            <span className="text-sm font-medium">{t(i18nKey)}</span>
          </button>
        );
      })}
    </div>
  );
}