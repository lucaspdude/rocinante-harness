"use client";

import { useEffect, useState } from "react";
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

interface Device {
  id: string;
  name: string;
  current: boolean;
  created_at: string;
  last_seen_at: string;
}

type Tab = "general" | "providers" | "developer" | "account" | "devices";

export default function SettingsPage() {
  const t = useT();
  const i18n = useI18n();
  const [tab, setTab] = useState<Tab>("general");
  const [devices, setDevices] = useState<Device[]>([]);
  const [theme, setTheme] = useState<"light" | "dark" | "system">("system");

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

  return (
    <>
      <TopNav />
      <main className="max-w-3xl mx-auto px-4 py-8">
        <h1 className="text-2xl font-semibold mb-6">{t("settings.title")}</h1>

        <div className="rh-tabs">
          <button
            type="button"
            className={`rh-tab ${tab === "general" ? "rh-tab-active" : ""}`}
            onClick={() => setTab("general")}
          >
            {t("settings.general")}
          </button>
          <button
            type="button"
            className={`rh-tab ${tab === "providers" ? "rh-tab-active" : ""}`}
            onClick={() => setTab("providers")}
          >
            {t("providers.title")}
          </button>
          <button
            type="button"
            className={`rh-tab ${tab === "account" ? "rh-tab-active" : ""}`}
            onClick={() => setTab("account")}
          >
            {t("settings.account")}
          </button>
          <button
            type="button"
            className={`rh-tab ${tab === "developer" ? "rh-tab-active" : ""}`}
            onClick={() => setTab("developer")}
          >
            {t("settings.developer")}
          </button>
          <button
            type="button"
            className={`rh-tab ${tab === "devices" ? "rh-tab-active" : ""}`}
            onClick={() => setTab("devices")}
          >
            {t("settings.devices")}
          </button>
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
            <GitSshPanel />
            <SshServerPanel />
            <CliToolsPanel />
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
