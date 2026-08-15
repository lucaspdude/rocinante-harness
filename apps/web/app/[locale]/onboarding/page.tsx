"use client";

import { useEffect, useState } from "react";
import { useT } from "../../../lib/i18n";
import { api } from "../../../lib/api/client";

interface OnboardingStatus {
  initialized: boolean;
  requires_setup: boolean;
  api_version: string;
}

export default function OnboardingPage() {
  const t = useT();
  const [status, setStatus] = useState<OnboardingStatus | null>(null);
  const [passphrase, setPassphrase] = useState("");
  const [confirm, setConfirm] = useState("");
  const [locale, setLocale] = useState<"en-US" | "pt-BR">("en-US");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.get<OnboardingStatus>("/api/v1/onboarding/status")
      .then(setStatus)
      .catch(() => setStatus({ initialized: false, requires_setup: true, api_version: "0.1.0" }));
  }, []);

  async function submit() {
    if (passphrase !== confirm) {
      setError("passphrases do not match");
      return;
    }
    if (passphrase.length < 8) {
      setError("passphrase must be at least 8 characters");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/v1/onboarding/init", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ passphrase, locale }),
      });
      if (!res.ok) {
        setError(`init failed: ${res.status}`);
      } else {
        window.location.href = `/${locale}/login`;
      }
    } finally {
      setBusy(false);
    }
  }

  if (!status) return <main><p>loading…</p></main>;

  if (status.initialized) {
    return (
      <main>
        <h1>{t("onboarding.complete")}</h1>
        <p>
          <a href={`/${locale}/login`}>{t("onboarding.goLogin")}</a>
        </p>
      </main>
    );
  }

  return (
    <main>
      <h1>{t("onboarding.title")}</h1>
      <p>{t("onboarding.subtitle")}</p>
      <label>
        {t("onboarding.passphrase")}
        <input
          type="password"
          value={passphrase}
          onChange={(e) => setPassphrase(e.target.value)}
          autoComplete="new-password"
        />
      </label>
      <label>
        {t("onboarding.confirm")}
        <input
          type="password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          autoComplete="new-password"
        />
      </label>
      <label>
        {t("onboarding.locale")}
        <select value={locale} onChange={(e) => setLocale(e.target.value as "en-US" | "pt-BR")}>
          <option value="en-US">en-US</option>
          <option value="pt-BR">pt-BR</option>
        </select>
      </label>
      {error && <p role="alert">{error}</p>}
      <button type="button" onClick={submit} disabled={busy}>
        {t("onboarding.submit")}
      </button>
    </main>
  );
}
