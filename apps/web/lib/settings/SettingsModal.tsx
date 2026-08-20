"use client";

// SettingsModal — PR-07 centered modal with left rail (4 sections).
//
// Replaces the 4-/5-tab settings page from phase-1/phase-3 with the
// DeepSeek-style "centered card, left rail nav, content area on the
// right" pattern (`docs/ui-ux-references/desktop.md §7`). The four
// sections reuse the existing 4 functional panels verbatim:
//
//   - General    → locale picker + theme picker (was `tab=general`)
//   - Providers  → ProvidersPanel (was `tab=providers`)
//   - Account    → Sign-out + device list (was `tab=account` + `tab=devices`)
//   - Developer  → Git SSH + SSH servers + CLI tools (was `tab=developer`
//                  with three sub-sections ssh / servers / clis)
//
// State model:
//   - Active section is read from the URL `?section=` query param on
//     mount, defaulting to `"general"`. Unknown values fall back to
//     `"general"` so an outdated deep-link never crashes the modal.
//   - Selecting a section in the left rail updates the URL via
//     `router.replace` (no history entry) — a refresh on /settings
//     always lands on the same section.
//   - `localStorage["rh:active-settings-section"]` persists the most
//     recently visited section for deep links that arrive without
//     a `?section=` (mirrors the legacy useActiveTab behavior so
//     `/settings` alone still feels sticky across redirects).
//
// Accessibility:
//   - `role="dialog"` + `aria-modal="true"`.
//   - Esc closes (calls `onClose`).
//   - Click on the backdrop closes (card click stops propagation).
//   - Initial focus lands on the close button so screen-reader
//     navigation begins inside the modal rather than on the
//     dimmed page underneath.

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useI18n, useT, useLocalizedPath } from "../i18n";
import { SUPPORTED_LOCALES, type Locale } from "../i18n/schema";
import { api } from "../api/client";
import { tokenStore } from "../auth/token-store";
import { ProvidersPanel } from "../providers/ProvidersPanel";
import { GitSshPanel } from "../ssh/GitSshPanel";
import { SshServerPanel } from "../ssh/SshServerPanel";
import { CliToolsPanel } from "../cli/CliToolsPanel";

export type SettingsSectionId = "general" | "providers" | "account" | "developer";

const SECTION_IDS: readonly SettingsSectionId[] = [
  "general",
  "providers",
  "account",
  "developer",
] as const;

const STORAGE_KEY = "rh:active-settings-section";
const DEFAULT_SECTION: SettingsSectionId = "general";

// Type guard — preserves narrowing in callers (SettingsSectionId vs string).
function isSectionId(v: string | null): v is SettingsSectionId {
  return v !== null && (SECTION_IDS as readonly string[]).includes(v);
}

interface Device {
  id: string;
  name: string;
  current: boolean;
  created_at: string;
  last_seen_at: string;
}

interface SettingsModalProps {
  onClose: () => void;
}

export function SettingsModal({ onClose }: SettingsModalProps) {
  const t = useT();
  const i18n = useI18n();
  const router = useRouter();
  const lp = useLocalizedPath();
  const searchParams = useSearchParams();
  const cardRef = useRef<HTMLDivElement | null>(null);
  const closeBtnRef = useRef<HTMLButtonElement | null>(null);

  // Resolve the initial section: ?section= > localStorage > default.
  const initialSection: SettingsSectionId = (() => {
    const fromQuery = searchParams.get("section");
    if (isSectionId(fromQuery)) return fromQuery;
    if (typeof window !== "undefined") {
      const stored = window.localStorage.getItem(STORAGE_KEY);
      if (isSectionId(stored)) return stored;
    }
    return DEFAULT_SECTION;
  })();

  const [section, setSectionState] = useState<SettingsSectionId>(initialSection);
  const [theme, setTheme] = useState<"light" | "dark" | "system">(() => {
    if (typeof window === "undefined") return "system";
    const stored = window.localStorage.getItem("rh-theme");
    return stored === "light" || stored === "dark" || stored === "system"
      ? stored
      : "system";
  });
  const [devices, setDevices] = useState<Device[]>([]);

  // Keep the URL in sync with the active section (router.replace —
  // no history entry so the back button does the natural thing).
  const setSection = useCallback(
    (next: SettingsSectionId) => {
      setSectionState(next);
      if (typeof window !== "undefined") {
        window.localStorage.setItem(STORAGE_KEY, next);
      }
      const params = new URLSearchParams(
        typeof window === "undefined" ? "" : window.location.search,
      );
      params.set("section", next);
      router.replace(lp(`/settings?${params.toString()}`));
    },
    [router, lp],
  );

  // Esc closes the modal. The handler is registered on `document`
  // because the modal's scrollable content lives in a sub-element
  // and we want Esc to work even if focus drifts outside the card.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.stopPropagation();
        onClose();
      }
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  // Initial focus on the close button so screen-reader navigation
  // starts inside the modal.
  useEffect(() => {
    closeBtnRef.current?.focus();
  }, []);

  // Lock body scroll while the modal is open so the dimmed page
  // underneath doesn't drift if the user overscrolls the modal.
  useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, []);

  // Device list — only used inside the Account section (was a
  // separate tab in phase-1). Loaded on mount so the section can
  // render immediately if the user navigates there.
  useEffect(() => {
    api
      .get<{ devices: Device[] }>("/api/v1/devices")
      .then((d) => setDevices(d.devices ?? []))
      .catch(() => {
        // Endpoint is best-effort; an empty list still renders fine.
      });
  }, []);

  async function logout() {
    try {
      await api.post("/api/v1/logout");
    } catch {
      // The api may reject (token already expired, etc.) — fall
      // through to clearing the local token regardless.
    }
    await tokenStore.clear();
    window.location.href = `/${i18n.locale}/login`;
  }

  const rail: Array<{
    id: SettingsSectionId;
    label: string;
    glyph: string;
  }> = [
    { id: "general", label: t("settings.general"), glyph: "G" },
    { id: "providers", label: t("providers.title"), glyph: "P" },
    { id: "account", label: t("settings.account"), glyph: "A" },
    { id: "developer", label: t("settings.developer"), glyph: "D" },
  ];

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="rh-settings-title"
      data-testid="rh-settings-modal"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={(e) => {
        // Click on backdrop (not bubbled from the card) closes.
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={cardRef}
        className="rh-card w-full max-w-3xl max-h-[90vh] flex flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-3 shrink-0">
          <h1
            id="rh-settings-title"
            className="text-base font-semibold"
          >
            {t("settings.title")}
          </h1>
          <button
            ref={closeBtnRef}
            type="button"
            aria-label={t("settings.close")}
            data-testid="rh-settings-close"
            onClick={onClose}
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] rounded p-1"
          >
            <span aria-hidden="true" className="text-lg leading-none">
              ×
            </span>
          </button>
        </header>

        <div className="flex flex-1 min-h-0">
          <aside
            aria-label="Settings sections"
            className="w-44 shrink-0 border-r border-[var(--color-border)] py-2 flex flex-col gap-0.5"
          >
            {rail.map((item) => {
              const active = section === item.id;
              return (
                <button
                  key={item.id}
                  type="button"
                  data-testid={`rh-settings-rail-${item.id}`}
                  data-active={active ? "true" : undefined}
                  onClick={() => setSection(item.id)}
                  aria-current={active ? "page" : undefined}
                  className={
                    "flex items-center gap-2.5 px-4 py-2 text-sm text-left transition-colors " +
                    (active
                      ? "bg-[var(--color-bg-subtle)] text-[var(--color-fg)] font-medium"
                      : "text-[var(--color-fg-muted)] hover:bg-[var(--color-bg-subtle)] hover:text-[var(--color-fg)]")
                  }
                >
                  <span
                    aria-hidden="true"
                    className={
                      "inline-flex items-center justify-center w-5 h-5 rounded text-[10px] font-mono " +
                      (active
                        ? "bg-[var(--color-primary)] text-white"
                        : "bg-[var(--color-bg)] border border-[var(--color-border)]")
                    }
                  >
                    {item.glyph}
                  </span>
                  <span className="truncate">{item.label}</span>
                </button>
              );
            })}
          </aside>

          <main className="flex-1 min-w-0 overflow-y-auto px-6 py-5">
            {section === "general" && (
              <GeneralSection
                theme={theme}
                setTheme={(next) => {
                  setTheme(next);
                  if (typeof window !== "undefined") {
                    window.localStorage.setItem("rh-theme", next);
                    document.documentElement.dataset.theme =
                      next === "system" ? "" : next;
                  }
                }}
                setLocale={i18n.setLocale}
              />
            )}
            {section === "providers" && <ProvidersPanel />}
            {section === "account" && (
              <AccountSection
                devices={devices}
                onRevoke={async (id) => {
                  await api.delete(`/api/v1/devices/${id}`);
                  setDevices((prev) => prev.filter((d) => d.id !== id));
                }}
                onLogout={() => void logout()}
              />
            )}
            {section === "developer" && (
              <DeveloperSection idPrefix="rh-settings-section-developer" />
            )}
          </main>
        </div>
      </div>
    </div>
  );
}

interface GeneralSectionProps {
  theme: "light" | "dark" | "system";
  setTheme: (next: "light" | "dark" | "system") => void;
  setLocale: (next: Locale) => void;
}

function GeneralSection({ theme, setTheme, setLocale }: GeneralSectionProps) {
  const t = useT();
  const i18n = useI18n();
  return (
    <div className="flex flex-col">
      <Row
        label={t("settings.locale")}
        description="Used across the UI; the api stores the choice per device."
      >
        <select
          id="set-locale"
          value={i18n.locale}
          onChange={(e) => setLocale(e.target.value as Locale)}
          className="rh-input w-44"
        >
          {SUPPORTED_LOCALES.map((l) => (
            <option key={l} value={l}>
              {l}
            </option>
          ))}
        </select>
      </Row>
      <Row
        label={t("settings.theme")}
        description="Light, dark, or follow the OS. The api applies the choice on every page render."
        last
      >
        <select
          id="set-theme"
          value={theme}
          onChange={(e) => setTheme(e.target.value as "light" | "dark" | "system")}
          className="rh-input w-44"
        >
          <option value="light">{t("settings.themeLight")}</option>
          <option value="dark">{t("settings.themeDark")}</option>
          <option value="system">{t("settings.themeSystem")}</option>
        </select>
      </Row>
    </div>
  );
}

interface AccountSectionProps {
  devices: Device[];
  onRevoke: (id: string) => Promise<void>;
  onLogout: () => void;
}

function AccountSection({ devices, onRevoke, onLogout }: AccountSectionProps) {
  const t = useT();
  return (
    <div className="flex flex-col gap-6">
      <Row
        label={t("settings.logout")}
        description="Revokes this device's bearer token and bounces to the login screen."
      >
        <button
          type="button"
          onClick={onLogout}
          data-testid="rh-settings-sign-out"
          className="rh-button-danger"
        >
          {t("settings.logout")}
        </button>
      </Row>

      <div>
        <h2 className="text-sm font-medium mb-2">{t("settings.devices")}</h2>
        {devices.length === 0 ? (
          <p className="text-xs text-[var(--color-fg-muted)] py-2">
            {t("settings.devicesEmpty")}
          </p>
        ) : (
          <ul className="flex flex-col gap-2">
            {devices.map((d) => (
              <li
                key={d.id}
                className="flex items-center justify-between px-3 py-2 rounded border border-[var(--color-border)]"
              >
                <span className="text-sm">
                  <strong>{d.name}</strong>
                  {d.current && (
                    <span className="ml-2 text-xs text-[var(--color-primary)]">
                      ({t("settings.current")})
                    </span>
                  )}
                </span>
                <button
                  type="button"
                  disabled={d.current}
                  onClick={() => void onRevoke(d.id)}
                  className="rh-button-ghost text-xs"
                >
                  {t("settings.revoke")}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

interface DeveloperSectionProps {
  idPrefix: string;
}

function DeveloperSection({ idPrefix }: DeveloperSectionProps) {
  return (
    <div className="flex flex-col gap-8">
      <div id={`${idPrefix}-ssh`}>
        <GitSshPanel />
      </div>
      <div id={`${idPrefix}-servers`}>
        <SshServerPanel />
      </div>
      <div id={`${idPrefix}-clis`}>
        <CliToolsPanel />
      </div>
    </div>
  );
}

interface RowProps {
  label: string;
  description?: string;
  children: React.ReactNode;
  last?: boolean;
}

// Row — visual primitive used across every settings section. Stable
// contract: horizontal row with label + description on the left,
// control on the right, separated from the next row by a thin border.
// `last` suppresses the trailing border so a section can sit flush
// against its container.
function Row({ label, description, children, last }: RowProps) {
  return (
    <div
      className={
        "flex items-center justify-between gap-4 py-3 " +
        (last ? "" : "border-b border-[var(--color-border)]")
      }
    >
      <div className="min-w-0">
        <div className="text-sm font-medium">{label}</div>
        {description && (
          <div className="text-xs text-[var(--color-fg-muted)] mt-0.5 max-w-md">
            {description}
          </div>
        )}
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  );
}
