"use client";

// Onboarding has 2 steps now:
//
//   1. Passphrase + locale (sets up the api's auth)
//   2. Provider credentials (informational; the api can store
//      the keys via the keystore + form, so this step helps
//      the user wire up at least one provider)
//
// Step 2 is skippable: many users will want to finish setting
// up the api first, then come back to providers later. The
// "Continue to sign in" link jumps to /login. After step 2,
// the user lands on /login as before.
//
// A StepDots indicator at the top of each card shows the user
// where they are in the flow.

import { useEffect, useState } from "react";
import { useT, useLocalizedPath } from "../../../lib/i18n";
import { api } from "../../../lib/api/client";
import { ProvidersPanel } from "../../../lib/providers/ProvidersPanel";
import { StepDots } from "../../../lib/components/StepDots";

interface OnboardingStatus {
  initialized: boolean;
  requires_setup: boolean;
  api_version: string;
}

type Step = "passphrase" | "providers";

const STEP_NAMES = ["passphrase", "providers"] as const;

export default function OnboardingPage() {
  const t = useT();
  const lp = useLocalizedPath();
  const [status, setStatus] = useState<OnboardingStatus | null>(null);
  const [passphrase, setPassphrase] = useState("");
  const [confirm, setConfirm] = useState("");
  const [locale, setLocale] = useState<"en-US" | "pt-BR">("en-US");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [step, setStep] = useState<Step>("passphrase");

  useEffect(() => {
    api
      .get<OnboardingStatus>("/api/v1/onboarding/status")
      .then(setStatus)
      .catch(() =>
        setStatus({
          initialized: false,
          requires_setup: true,
          api_version: "0.1.0",
        })
      );
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
        return;
      }
      setStep("providers");
    } finally {
      setBusy(false);
    }
  }

  if (!status) {
    return (
      <main className="min-h-screen flex items-center justify-center px-4">
        <p className="text-[var(--color-fg-muted)]">{t("common.loading")}</p>
      </main>
    );
  }

  if (status.initialized) {
    return (
      <main className="min-h-screen flex items-center justify-center px-4">
        <div className="rh-card text-center">
          <h1 className="text-2xl font-semibold mb-3">
            {t("onboarding.complete")}
          </h1>
          <a href={lp("/login")} className="rh-button-primary inline-block">
            {t("onboarding.goLogin")}
          </a>
        </div>
      </main>
    );
  }

  if (step === "providers") {
    const stepLabel = (i: number, _n: number, name: string) => {
      const map: Record<string, string> = {
        passphrase: t("onboarding.passphrase"),
        providers: t("providers.title"),
      };
      return `${i + 1}. ${map[name] ?? name}`;
    };
    return (
      <main className="min-h-screen flex items-center justify-center px-4 py-12">
        <div className="w-full max-w-2xl">
          <StepDots
            steps={STEP_NAMES.map((s) =>
              s === "passphrase" ? t("onboarding.passphrase") : t("providers.title")
            )}
            active={1}
            stepLabel={stepLabel}
          />
          <ProvidersPanel />
          <div className="mt-6 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => setStep("passphrase")}
              className="rh-button-ghost"
            >
              ← {t("onboarding.passphrase")}
            </button>
            <a
              href={lp("/login")}
              className="rh-button-primary inline-block"
            >
              {t("onboarding.goLogin")}
            </a>
          </div>
        </div>
      </main>
    );
  }

  const stepLabel = (i: number, _n: number, name: string) => {
    const map: Record<string, string> = {
      passphrase: t("onboarding.passphrase"),
      providers: t("providers.title"),
    };
    return `${i + 1}. ${map[name] ?? name}`;
  };

  return (
    <main className="min-h-screen flex items-center justify-center px-4">
      <div className="w-full max-w-md">
        <StepDots
          steps={STEP_NAMES.map((s) =>
            s === "passphrase" ? t("onboarding.passphrase") : t("providers.title")
          )}
          active={0}
          stepLabel={stepLabel}
        />
        <div className="rh-card">
          <h1 className="text-2xl font-semibold mb-2">
            {t("onboarding.title")}
          </h1>
          <p className="text-[var(--color-fg-muted)] mb-6">
            {t("onboarding.subtitle")}
          </p>
          <div className="flex flex-col gap-4">
            <div>
              <label className="rh-label" htmlFor="onb-pass">
                {t("onboarding.passphrase")}
              </label>
              <input
                id="onb-pass"
                type="password"
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                autoComplete="new-password"
                className="rh-input"
              />
            </div>
            <div>
              <label className="rh-label" htmlFor="onb-confirm">
                {t("onboarding.confirm")}
              </label>
              <input
                id="onb-confirm"
                type="password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                autoComplete="new-password"
                className="rh-input"
              />
            </div>
            <div>
              <label className="rh-label" htmlFor="onb-locale">
                {t("onboarding.locale")}
              </label>
              <select
                id="onb-locale"
                value={locale}
                onChange={(e) =>
                  setLocale(e.target.value as "en-US" | "pt-BR")
                }
                className="rh-input"
              >
                <option value="en-US">en-US</option>
                <option value="pt-BR">pt-BR</option>
              </select>
            </div>
            {error && (
              <p role="alert" className="rh-error">
                {error}
              </p>
            )}
            <button
              type="button"
              onClick={submit}
              disabled={busy}
              className="rh-button-primary"
            >
              {t("onboarding.submit")}
            </button>
          </div>
        </div>
      </div>
    </main>
  );
}
