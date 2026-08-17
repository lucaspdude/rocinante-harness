"use client";

// Onboarding has 3 steps, in this order:
//
//   1. Provider key (gating) — the user pastes at least one
//      provider's API key. Without a key, omp can't talk to a
//      model, so no other step makes sense. This step calls
//      POST /api/v1/providers/{name}/key (public, no auth) and
//      the api writes the value to its keystore. The api
//      process does NOT need to restart for the keystore write
//      to take effect — the manager reads the file on every
//      session spawn.
//
//   2. Passphrase + locale — calls POST /api/v1/onboarding/init
//      to create .ed25519 + .ed25519.bak + the SQLite schema.
//      The api then self-restarts (250 ms goroutine) so the
//      newly written key file is loaded on the next start.
//
//   3. Done — links to /login.
//
// Step 1 is mandatory: the "Continue" button stays disabled
// until at least one provider shows the green "Configured"
// dot. Step 2 stays disabled until the user has visited step
// 1 (so a curious user who lands directly on /onboarding
// can't skip past it).
//
// If the api's onboarding/status already reports initialized
// (e.g. the installer pre-initialized the key), step 1
// auto-marks the configured providers green and lets the
// user advance without re-pasting keys.

import { useEffect, useMemo, useState } from "react";
import { useT, useLocalizedPath } from "../../../lib/i18n";
import { api } from "../../../lib/api/client";
import { ProvidersPanel } from "../../../lib/providers/ProvidersPanel";
import { StepDots } from "../../../lib/components/StepDots";

interface OnboardingStatus {
  initialized: boolean;
  requires_setup: boolean;
  api_version: string;
}

type Step = "providers" | "passphrase" | "done";

const STEP_KEYS = ["providers", "passphrase", "done"] as const;

export default function OnboardingPage() {
  const t = useT();
  const lp = useLocalizedPath();
  const [status, setStatus] = useState<OnboardingStatus | null>(null);
  const [passphrase, setPassphrase] = useState("");
  const [confirm, setConfirm] = useState("");
  const [locale, setLocale] = useState<"en-US" | "pt-BR">("en-US");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [step, setStep] = useState<Step>("providers");
  // Whether at least one provider is currently configured.
  // Tracked locally so the gate updates without a round-trip
  // to /api/v1/meta on every keystore write.
  const [providerCount, setProviderCount] = useState(0);
  // After init, /api/v1/onboarding/status reports initialized=true.
  // We cache that signal so step 3 renders without re-fetching.
  const [initSucceeded, setInitSucceeded] = useState(false);

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

  const stepIndex = useMemo(() => STEP_KEYS.indexOf(step), [step]);
  const canAdvanceFromProviders = providerCount > 0;
  const initialized = status?.initialized === true || initSucceeded;

  async function submitPassphrase() {
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
      setInitSucceeded(true);
      setStep("done");
    } finally {
      setBusy(false);
    }
  }

  // Loading state — api status not yet known.
  if (!status) {
    return (
      <main className="min-h-screen flex items-center justify-center px-4">
        <p className="text-[var(--color-fg-muted)]">{t("common.loading")}</p>
      </main>
    );
  }

  // If the api is already initialized (e.g. installer pre-init'd
  // the key, or the user is returning to an already-onboarded
  // install), skip straight to the "Done" step. There's no
  // point re-prompting for a passphrase we already have.
  if (initialized && step !== "done" && !initSucceeded) {
    return (
      <main className="min-h-screen flex items-center justify-center px-4 py-12">
        <div className="w-full max-w-2xl">
          <StepDots
            steps={STEP_KEYS.map((s) => stepLabel(t, s))}
            active={2}
          />
          <DoneCard
            t={t}
            lp={lp}
          />
        </div>
      </main>
    );
  }

  if (step === "providers") {
    return (
      <main className="min-h-screen flex items-center justify-center px-4 py-12">
        <div className="w-full max-w-2xl">
          <StepDots
            steps={STEP_KEYS.map((s) => stepLabel(t, s))}
            active={0}
          />
          <h1 className="text-2xl font-semibold mb-1">
            {t("onboarding.stepProviders.title")}
          </h1>
          <p className="text-[var(--color-fg-muted)] text-sm mb-4">
            {t("onboarding.stepProviders.subtitle")}
          </p>
          <ProvidersPanel onConfiguredCountChange={setProviderCount} />
          <div className="mt-6 flex items-center justify-between gap-3">
            <p
              className={
                canAdvanceFromProviders
                  ? "text-xs text-[var(--color-fg-muted)]"
                  : "text-xs text-[var(--color-fg-muted)] italic"
              }
            >
              {canAdvanceFromProviders
                ? t("onboarding.alreadyConfiguredSkip")
                : t("onboarding.gateNeedProvider")}
            </p>
            <button
              type="button"
              onClick={() => setStep("passphrase")}
              disabled={!canAdvanceFromProviders}
              className="rh-button-primary disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {t("onboarding.stepProviders.cta")} →
            </button>
          </div>
        </div>
      </main>
    );
  }

  if (step === "passphrase") {
    return (
      <main className="min-h-screen flex items-center justify-center px-4 py-12">
        <div className="w-full max-w-md">
          <StepDots
            steps={STEP_KEYS.map((s) => stepLabel(t, s))}
            active={1}
          />
          <h1 className="text-2xl font-semibold mb-1">
            {t("onboarding.stepPassphrase.title")}
          </h1>
          <p className="text-[var(--color-fg-muted)] text-sm mb-6">
            {t("onboarding.stepPassphrase.subtitle")}
          </p>
          <div className="rh-card">
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
            </div>
          </div>
          <div className="mt-6 flex items-center justify-between gap-3">
            <button
              type="button"
              onClick={() => setStep("providers")}
              className="rh-button-ghost"
            >
              ← {t("providers.title")}
            </button>
            <button
              type="button"
              onClick={submitPassphrase}
              disabled={busy}
              className="rh-button-primary"
            >
              {busy ? t("common.loading") : t("onboarding.submit")}
            </button>
          </div>
        </div>
      </main>
    );
  }

  // step === "done"
  return (
    <main className="min-h-screen flex items-center justify-center px-4 py-12">
      <div className="w-full max-w-2xl">
        <StepDots
          steps={STEP_KEYS.map((s) => stepLabel(t, s))}
          active={2}
        />
        <DoneCard t={t} lp={lp} />
      </div>
    </main>
  );
}

function stepLabel(
  t: (k: string) => string,
  key: (typeof STEP_KEYS)[number]
): string {
  switch (key) {
    case "providers":
      return t("onboarding.stepProviders.title").replace(/^Step \d+\.\s*/, "");
    case "passphrase":
      return t("onboarding.stepPassphrase.title").replace(/^Step \d+\.\s*/, "");
    case "done":
      return t("onboarding.stepDone.title");
  }
}

function DoneCard({
  t,
  lp,
}: {
  t: (k: string) => string;
  lp: (p: string) => string;
}) {
  return (
    <div className="rh-card text-center">
      <h1 className="text-2xl font-semibold mb-3">
        {t("onboarding.stepDone.title")}
      </h1>
      <p className="text-[var(--color-fg-muted)] mb-6">
        {t("onboarding.stepDone.subtitle")}
      </p>
      <a href={lp("/login")} className="rh-button-primary inline-block">
        {t("onboarding.goLogin")}
      </a>
    </div>
  );
}
