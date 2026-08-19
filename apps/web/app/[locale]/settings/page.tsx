"use client";

import { useEffect, useRef, useState } from "react";
import { useT, useI18n } from "../../../lib/i18n";
import { api } from "../../../lib/api/client";
import { tokenStore } from "../../../lib/auth/token-store";
import { ProvidersPanel } from "../../../lib/providers/ProvidersPanel";
import { GitSshPanel } from "../../../lib/ssh/GitSshPanel";
import { SshServerPanel } from "../../../lib/ssh/SshServerPanel";
import { TopNav } from "../../../lib/components/TopNav";
import { CliToolsPanel } from "../../../lib/cli/CliToolsPanel";
import {
  SUPPORTED_LOCALES,
  type Locale,
} from "../../../lib/i18n/schema";
import {
  useActiveTab,
  type SubId,
  type TabId,
} from "../../../lib/settings/useActiveTab";

interface Device {
  id: string;
  name: string;
  current: boolean;
  created_at: string;
  last_seen_at: string;
}

const TAB_ORDER: TabId[] = [
  "general",
  "providers",
  "account",
  "developer",
  "devices",
];

export default function SettingsPage() {
  const t = useT();
  const i18n = useI18n();
  const { tab, sub, setTab, isReady } = useActiveTab();
  const [devices, setDevices] = useState<Device[]>([]);
  const [theme, setTheme] = useState<"light" | "dark" | "system">("system");

  // Refs to each developer-tools sub-section. Used by the
  // scroll-into-view effect below to anchor a deep link
  // (?tab=developer&sub=clis) on the right sub-section.
  const subRefs = useRef<Record<SubId, HTMLDivElement | null>>({
    ssh: null,
    servers: null,
    clis: null,
  });

  useEffect(() => {
    api
      .get<{ devices: Device[] }>("/api/v1/devices")
      .then((d) => setDevices(d.devices ?? []))
      .catch(() => {});
    const stored = (typeof window !== "undefined" &&
      window.localStorage.getItem("rh-theme")) as
      | "light"
      | "dark"
      | "system"
      | null;
    if (stored) setTheme(stored);
  }, []);

  // Scroll the active sub-section (or the tab strip, for non-developer
  // tabs) into view whenever the URL-driven state changes. Wrapped in
  // rAF + a small safety timeout to make sure the DOM has the new
  // sub-section mounted before scrolling — see PR-10 spec §5.2.
  useEffect(() => {
    if (!isReady) return;
    const targetId =
      tab === "developer" ? `rh-settings-sub-${sub}` : `rh-settings-tabs`;
    const run = () => {
      const el = document.getElementById(targetId);
      if (el) {
        el.scrollIntoView({ behavior: "smooth", block: "start" });
      }
    };
    const raf = requestAnimationFrame(() => {
      run();
      // Safety net: if the developer tab just opened, the sub-section
      // element may mount on the next paint. Retry once after 50ms.
      window.setTimeout(run, 50);
    });
    return () => cancelAnimationFrame(raf);
  }, [tab, sub, isReady]);

  function setAndPersistTheme(next: "light" | "dark" | "system") {
    setTheme(next);
    if (typeof window !== "undefined") {
      window.localStorage.setItem("rh-theme", next);
      document.documentElement.dataset.theme =
        next === "system" ? "" : next;
    }
  }

  function setAndPersistLocale(next: Locale) {
    i18n.setLocale(next);
  }

  async function logout() {
    try {
      await api.post("/api/v1/logout");
    } catch {
      // Even if the api rejects (e.g. token already expired),
      // drop the local token and bounce to the login page.
    }
    await tokenStore.clear();
    window.location.href = `/${i18n.locale}/login`;
  }

  function setTabAndMaybeSub(next: TabId) {
    if (next === "developer") setTab(next, sub);
    else setTab(next);
  }

  return (
    <>
      <TopNav />
      <main className="max-w-3xl mx-auto px-4 py-8">
        <h1 className="text-2xl font-semibold mb-6">{t("settings.title")}</h1>

        <div id="rh-settings-tabs" className="rh-tabs">
          {TAB_ORDER.map((id) => (
            <button
              key={id}
              type="button"
              className={`rh-tab ${tab === id ? "rh-tab-active" : ""}`}
              onClick={() => setTabAndMaybeSub(id)}
            >
              {t(
                id === "providers"
                  ? "providers.title"
                  : `settings.${id}`,
              )}
            </button>
          ))}
        </div>

        {tab === "general" && (
          <section className="rh-card flex flex-col gap-4">
            <div>
              <label className="rh-label" htmlFor="set-locale">
                {t("settings.locale")}
              </label>
              <select
                id="set-locale"
                value={i18n.locale}
                onChange={(e) => setAndPersistLocale(e.target.value as Locale)}
                className="rh-input"
              >
                {SUPPORTED_LOCALES.map((l) => (
                  <option key={l} value={l}>
                    {l}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="rh-label" htmlFor="set-theme">
                {t("settings.theme")}
              </label>
              <select
                id="set-theme"
                value={theme}
                onChange={(e) =>
                  setAndPersistTheme(e.target.value as "light" | "dark" | "system")
                }
                className="rh-input"
              >
                <option value="light">{t("settings.themeLight")}</option>
                <option value="dark">{t("settings.themeDark")}</option>
                <option value="system">{t("settings.themeSystem")}</option>
              </select>
            </div>
          </section>
        )}

        {tab === "providers" && <ProvidersPanel />}

        {tab === "developer" && (
          <div className="flex flex-col gap-8">
            <div
              id="rh-settings-sub-ssh"
              ref={(el) => {
                subRefs.current.ssh = el;
              }}
            >
              <GitSshPanel />
            </div>
            <div
              id="rh-settings-sub-servers"
              ref={(el) => {
                subRefs.current.servers = el;
              }}
            >
              <SshServerPanel />
            </div>
            <div
              id="rh-settings-sub-clis"
              ref={(el) => {
                subRefs.current.clis = el;
              }}
            >
              <CliToolsPanel />
            </div>
          </div>
        )}

        {tab === "account" && (
          <section className="rh-card flex flex-col gap-4">
            <h2 className="text-lg font-medium">{t("settings.account")}</h2>
            <button
              type="button"
              onClick={logout}
              className="rh-button-danger self-start"
            >
              {t("settings.logout")}
            </button>
          </section>
        )}

        {tab === "devices" && (
          <section className="rh-card flex flex-col gap-3">
            <h2 className="text-lg font-medium">{t("settings.devices")}</h2>
            {devices.length === 0 ? (
              <p className="text-[var(--color-fg-muted)]">
                {t("settings.devicesEmpty")}
              </p>
            ) : (
              <ul className="flex flex-col gap-2">
                {devices.map((d) => (
                  <li
                    key={d.id}
                    className="flex items-center justify-between px-3 py-2 rounded border border-[var(--color-border)]"
                  >
                    <span>
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
                      onClick={async () => {
                        await api.delete(`/api/v1/devices/${d.id}`);
                        setDevices((prev) =>
                          prev.filter((x) => x.id !== d.id)
                        );
                      }}
                      className="rh-button-ghost"
                    >
                      {t("settings.revoke")}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </section>
        )}
      </main>
    </>
  );
}
