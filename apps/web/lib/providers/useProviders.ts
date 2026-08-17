"use client";

// Hook that polls /api/v1/meta and returns the set of detected
// provider env vars. The api never returns the values, only the
// booleans — the web UI uses these to render a "Configured /
// Not set" checklist in the Providers settings tab and in the
// onboarding step.
//
// Polling is 5 s by default. The status is read-only on the web
// side: the actual env-var write happens in /etc/roc-harness/env
// on the host (see Settings → Providers for instructions).

import { useEffect, useState } from "react";
import { api } from "../api/client";

export interface ProviderStatus {
  anthropic: boolean;
  openai: boolean;
  gemini: boolean;
  openrouter: boolean;
  minimax_token_plan: boolean;
}

export interface ProviderDef {
  key: keyof ProviderStatus;
  label: string;
  envVar: string;
  installHint: string;
}

export const PROVIDERS: ProviderDef[] = [
  {
    key: "anthropic",
    label: "Anthropic",
    envVar: "ANTHROPIC_API_KEY",
    installHint: "console.anthropic.com → Settings → API Keys",
  },
  {
    key: "openai",
    label: "OpenAI",
    envVar: "OPENAI_API_KEY",
    installHint: "platform.openai.com → API keys",
  },
  {
    key: "gemini",
    label: "Gemini",
    envVar: "GEMINI_API_KEY",
    installHint: "aistudio.google.com → API keys",
  },
  {
    key: "openrouter",
    label: "OpenRouter",
    envVar: "OPENROUTER_API_KEY",
    installHint: "openrouter.ai → Keys",
  },
  {
    key: "minimax_token_plan",
    label: "Minimax (token plan)",
    envVar: "MINIMAX_TOKEN_PLAN_API_KEY",
    installHint: "MiniMax dashboard → Token plan → API key",
  },
];

interface MetaResponse {
  api_version: string;
  omp_version: string;
  protocol_version: number;
  omp_bin: string;
  providers: ProviderStatus;
}

export function useProviders(intervalMs = 5000): {
  status: ProviderStatus | null;
  meta: Omit<MetaResponse, "providers"> | null;
  error: string | null;
  reload: () => void;
} {
  const [status, setStatus] = useState<ProviderStatus | null>(null);
  const [meta, setMeta] = useState<Omit<MetaResponse, "providers"> | null>(
    null
  );
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await api.get<MetaResponse>("/api/v1/meta");
        if (cancelled) return;
        setStatus(res.providers);
        setMeta({
          api_version: res.api_version,
          omp_version: res.omp_version,
          protocol_version: res.protocol_version,
          omp_bin: res.omp_bin,
        });
        setError(null);
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
      }
    }
    load();
    const id = setInterval(load, intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [intervalMs, tick]);

  return { status, meta, error, reload: () => setTick((n) => n + 1) };
}
