"use client";

import { useEffect, useState } from "react";
import { useT, useI18n } from "../../../lib/i18n";
import { api } from "../../../lib/api/client";
import { tokenStore } from "../../../lib/auth/token-store";
import { SUPPORTED_LOCALES, DEFAULT_LOCALE, type Locale } from "../../../lib/i18n/schema";

interface Device {
  id: string;
  name: string;
  current: boolean;
  created_at: string;
  last_seen_at: string;
}

export default function SettingsPage() {
  const t = useT();
  const i18n = useI18n();
  const [devices, setDevices] = useState<Device[]>([]);
  const [theme, setTheme] = useState<"light" | "dark" | "system">("system");

  useEffect(() => {
    api.get<{ devices: Device[] }>("/api/v1/devices")
      .then((d) => setDevices(d.devices ?? []))
      .catch(() => {});
    const stored = (typeof window !== "undefined" && window.localStorage.getItem("rh-theme")) as "light" | "dark" | "system" | null;
    if (stored) setTheme(stored);
  }, []);

  function setAndPersistTheme(next: "light" | "dark" | "system") {
    setTheme(next);
    if (typeof window !== "undefined") {
      window.localStorage.setItem("rh-theme", next);
      document.documentElement.dataset.theme = next;
    }
  }

  function setAndPersistLocale(next: Locale) {
    i18n.setLocale(next);
  }

  async function logout() {
    await api.post("/api/v1/logout", {}).catch(() => {});
    await tokenStore.clear();
    window.location.href = `/${i18n.locale}/login`;
  }

  return (
    <main>
      <h1>{t("settings.title")}</h1>

      <section>
        <h2>{t("settings.general")}</h2>
        <label>
          {t("settings.locale")}
          <select
            value={i18n.locale}
            onChange={(e) => setAndPersistLocale(e.target.value as Locale)}
          >
            {SUPPORTED_LOCALES.map((l) => (
              <option key={l} value={l}>
                {l}
              </option>
            ))}
          </select>
        </label>
        <label>
          {t("settings.theme")}
          <select value={theme} onChange={(e) => setAndPersistTheme(e.target.value as "light" | "dark" | "system")}>
            <option value="light">{t("settings.themeLight")}</option>
            <option value="dark">{t("settings.themeDark")}</option>
            <option value="system">{t("settings.themeSystem")}</option>
          </select>
        </label>
      </section>

      <section>
        <h2>{t("settings.account")}</h2>
        <button type="button" onClick={logout}>
          {t("settings.logout")}
        </button>
      </section>

      <section>
        <h2>{t("settings.devices")}</h2>
        {devices.length === 0 ? (
          <p>{t("settings.devicesEmpty")}</p>
        ) : (
          <ul>
            {devices.map((d) => (
              <li key={d.id}>
                <strong>{d.name}</strong>
                {d.current ? ` (${t("settings.current")})` : ""}
                <button
                  type="button"
                  onClick={async () => {
                    await api.delete(`/api/v1/devices/${d.id}`);
                    setDevices((prev) => prev.filter((x) => x.id !== d.id));
                  }}
                  disabled={d.current}
                >
                  {t("settings.revoke")}
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}
