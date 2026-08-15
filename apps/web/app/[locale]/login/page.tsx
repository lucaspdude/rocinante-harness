"use client";

import { useState } from "react";
import { useT } from "../../../lib/i18n";
import { api } from "../../../lib/api/client";
import { tokenStore } from "../../../lib/auth/token-store";

interface LoginResponse {
  access_token: string;
  refresh_token: string;
  device_id: string;
}

export default function LoginPage() {
  const t = useT();
  const [passphrase, setPassphrase] = useState("");
  const [deviceName, setDeviceName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const res = await api.post<LoginResponse>(
        "/api/v1/login",
        {
          json: { passphrase, device_name: deviceName },
          unauthenticated: true,
        }
      );
      if (res) {
        await tokenStore.save({
          access_token: res.access_token,
          refresh_token: res.refresh_token,
          device_id: res.device_id,
        });
        window.location.href = "/";
      }
    } catch (err: unknown) {
      const code = (err as { body?: { code?: string } }).body?.code;
      if (code === "auth_invalid_passphrase") {
        setError(t("login.invalidPassphrase"));
      } else {
        setError(t("login.networkError"));
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <main>
      <h1>{t("login.title")}</h1>
      <p>{t("login.subtitle")}</p>
      <form onSubmit={onSubmit}>
        <label htmlFor="passphrase">{t("login.passphrase")}</label>
        <input
          id="passphrase"
          name="passphrase"
          type="password"
          autoComplete="current-password"
          value={passphrase}
          onChange={(e) => setPassphrase(e.target.value)}
          required
        />
        <label htmlFor="deviceName">{t("login.deviceName")}</label>
        <input
          id="deviceName"
          name="deviceName"
          type="text"
          value={deviceName}
          onChange={(e) => setDeviceName(e.target.value)}
        />
        {error && <p role="alert">{error}</p>}
        <button type="submit" disabled={busy}>
          {t("login.submit")}
        </button>
      </form>
    </main>
  );
}
