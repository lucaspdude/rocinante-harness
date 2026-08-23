"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { useI18n, useT, useLocalizedPath } from "../../../lib/i18n";
import { api, ApiClientError } from "../../../lib/api/client";
import { tokenStore } from "../../../lib/auth/token-store";
import { useToast } from "../../../lib/toast";

interface LoginResponse {
  access: string;
  refresh: string;
  device_id: string;
}

// Phase 7 — item 01: validate a `?next=` query value as a safe
// same-origin, locale-prefixed path. Rejects cross-origin
// redirects (e.g. `?next=https://evil.example/...`) and
// wrong-locale redirects (e.g. `/en-US/...` when locale is
// `pt-BR`). On any error we fall through to the default landing
// (`/agent/new`).
function safeNextPath(next: string | null, locale: string): string | null {
  if (!next) return null;
  try {
    const u = new URL(next, window.location.origin);
    if (u.origin !== window.location.origin) return null;
    if (!u.pathname.startsWith(`/${locale}/`)) return null;
    return u.pathname + u.search + u.hash;
  } catch {
    return null;
  }
}

export default function LoginPage() {
  const t = useT();
  const i18n = useI18n();
  const lp = useLocalizedPath();
  const toast = useToast();
  const searchParams = useSearchParams();
  const [passphrase, setPassphrase] = useState("");
  const [deviceName, setDeviceName] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await api.post<LoginResponse>(
        "/api/v1/login",
        {
          json: { passphrase, device_name: deviceName },
          // Phase 7 — item 03 (cross-item contract): login must
          // not emit `rh:auth:expired` on 401, otherwise the
          // AuthExpiredListener reloads /login in a loop.
          unauthenticated: true,
        },
      );
      if (res) {
        await tokenStore.save({
          access_token: res.access,
          refresh_token: res.refresh,
          device_id: res.device_id,
        });
        // Send the user straight into a new chat session. The
        // home page is just a landing; logging in implies they
        // want to use the product. Phase 7 — item 01: when the
        // URL carries `?next=` (set by the home page redirect
        // for returning users), honour it instead so the user
        // lands on the originally-requested route.
        const next = safeNextPath(searchParams.get("next"), i18n.locale);
        window.location.href = next ?? lp("/agent/new");
      }
    } catch (err: unknown) {
      const code =
        err instanceof ApiClientError ? err.body.code : undefined;
      if (code === "auth_invalid_passphrase") {
        toast.error(t("login.invalidPassphrase"));
      } else {
        toast.error(t("login.networkError"));
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
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                required
                autoComplete="current-password"
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
