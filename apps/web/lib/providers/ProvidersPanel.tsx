"use client";

// Reusable Providers panel. Used by:
//   - Settings → Providers tab
//   - Onboarding step 2 (after passphrase)
//
// Renders a checklist of the 5 supported providers, indicating
// which ones are configured (env var set in the api process).
// Below the checklist: a one-line "How to set" guide that
// explains the user has to edit /etc/roc-harness/env on the
// host and restart roc-harness-api. We do not provide an input
// field for the key — the web never handles the key directly.

import { useState } from "react";
import { useT } from "../i18n";
import { PROVIDERS, useProviders, type ProviderDef } from "./useProviders";

export function ProvidersPanel() {
  const t = useT();
  const { status, error, reload } = useProviders(5000);
  const [copied, setCopied] = useState<string | null>(null);

  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(text);
      setTimeout(() => setCopied(null), 1500);
    } catch {
      // Clipboard API may be denied in some browsers; fall back
      // to selecting the text. For the MVP the copy is a nice
      // to have, not a blocker.
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h2 className="text-lg font-medium mb-1">{t("providers.title")}</h2>
        <p className="text-sm text-[var(--color-fg-muted)]">
          {t("providers.subtitle")}
        </p>
      </div>

      <div className="rh-card">
        <h3 className="text-sm font-medium mb-3">{t("providers.checklist")}</h3>
        {error && (
          <p role="alert" className="rh-error mb-3">
            {error}
          </p>
        )}
        <ul className="flex flex-col gap-2">
          {PROVIDERS.map((p) => (
            <ProviderRow
              key={p.key}
              p={p}
              configured={status?.[p.key] ?? false}
            />
          ))}
        </ul>
        <button
          type="button"
          onClick={reload}
          className="rh-button-ghost mt-3 text-sm"
        >
          {t("common.loading")}…
        </button>
      </div>

      <div className="rh-card">
        <h3 className="text-sm font-medium mb-2">{t("providers.howToSet")}</h3>
        <p className="text-sm text-[var(--color-fg-muted)] mb-3">
          {t("providers.envFile")}
        </p>
        <CodeBlock
          text={`# /etc/roc-harness/env  (chmod 0600, owned by root)
ROCINANTE_PASSPHRASE=...
OMP_BIN=/root/.local/share/rocinante-harness/bin/omp
MINIMAX_TOKEN_PLAN_API_KEY=sk-cp-...`}
          onCopy={() => copy("ROCINANTE_PASSPHRASE=...\nOMP_BIN=...\nMINIMAX_TOKEN_PLAN_API_KEY=sk-cp-...")}
          copied={!!copied}
        />
        <p className="text-sm text-[var(--color-fg-muted)] mt-3 mb-1">
          {t("providers.restartHint")}
        </p>
        <CodeBlock
          text="systemctl restart roc-harness-api"
          onCopy={() => copy("systemctl restart roc-harness-api")}
          copied={!!copied}
        />
        <p className="text-xs text-[var(--color-fg-muted)] mt-3">
          {t("providers.envReloadNote")}
        </p>
        <p className="text-sm text-[var(--color-fg-muted)] mt-4 mb-1">
          {t("providers.terminalAlt")}
        </p>
        <CodeBlock
          text={`omp /login  # paste the key once, stored in ~/.omp/agent/agent.db`}
          onCopy={() => copy("omp /login")}
          copied={!!copied}
        />
      </div>
    </div>
  );
}

function ProviderRow({
  p,
  configured,
}: {
  p: ProviderDef;
  configured: boolean;
}) {
  const t = useT();
  return (
    <li className="flex items-center gap-3 py-1">
      <span
        aria-hidden="true"
        className={
          configured
            ? "inline-block w-2.5 h-2.5 rounded-full bg-green-500"
            : "inline-block w-2.5 h-2.5 rounded-full bg-zinc-500/40"
        }
      />
      <div className="flex-1 min-w-0">
        <div className="font-medium">{p.label}</div>
        <div className="text-xs text-[var(--color-fg-muted)] font-mono">
          {p.envVar}
        </div>
      </div>
      <span
        className={
          configured
            ? "text-xs px-2 py-0.5 rounded-full bg-green-500/15 text-green-600 dark:text-green-400"
            : "text-xs px-2 py-0.5 rounded-full bg-zinc-500/10 text-[var(--color-fg-muted)]"
        }
      >
        {configured ? t("providers.configured") : t("providers.missing")}
      </span>
    </li>
  );
}

function CodeBlock({
  text,
  onCopy,
  copied,
}: {
  text: string;
  onCopy: () => void;
  copied: boolean;
}) {
  const t = useT();
  return (
    <div className="relative">
      <pre className="text-xs bg-zinc-900/80 text-zinc-100 rounded-md p-3 overflow-x-auto">
        {text}
      </pre>
      <button
        type="button"
        onClick={onCopy}
        className="absolute top-2 right-2 text-xs px-2 py-0.5 rounded bg-zinc-700/80 text-zinc-100 hover:bg-zinc-600"
      >
        {copied ? t("providers.copied") : t("providers.copyHint")}
      </button>
    </div>
  );
}
