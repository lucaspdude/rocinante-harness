"use client";

import { useState } from "react";
import { useT, useLocalizedPath } from "../../../lib/i18n";
import { api } from "../../../lib/api/client";
import { tokenStore } from "../../../lib/auth/token-store";

interface LoginResponse {
  access: string;
  refresh: string;
  device_id: string;
}

export default function LoginPage() {
  const t = useT();
  const lp = useLocalizedPath();
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
          access_token: res.access,
          refresh_token: res.refresh,
          device_id: res.device_id,
        });
        // Send the user straight into a new chat session. The
        // home page is just a landing; logging in implies they
        // want to use the product.
        window.location.href = lp("/agent/new");
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
    <main className="min-h-screen flex items-center justify-center px-4">
      <div className="w-full max-w-md">
        <div className="rh-card">
          <h1 className="text-2xl font-semibold mb-2">{t("login.title")}</h1>
          <p className="text-[var(--color-fg-muted)] mb-6">
            {t("login.subtitle")}
          </p>
          <form onSubmit={onSubmit} className="flex flex-col gap-4">
            <div>
              <label className="rh-label" htmlFor="passphrase">
                {t("login.passphrase")}
              </label>
              <input
                id="passphrase"
                name="passphrase"
                type="password"
                autoComplete="current-password"
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                required
                className="rh-input"
              />
            </div>
            <div>
              <label className="rh-label" htmlFor="deviceName">
                {t("login.deviceName")}
              </label>
              <input
                id="deviceName"
                name="deviceName"
                type="text"
                value={deviceName}
                onChange={(e) => setDeviceName(e.target.value)}
                className="rh-input"
              />
            </div>
            {error && (
              <p role="alert" className="rh-error">
                {error}
              </p>
            )}
            <button
              type="submit"
              disabled={busy}
              className="rh-button-primary"
            >
              {t("login.submit")}
            </button>
          </form>
        </div>
      </div>
    </main>
  );
}
