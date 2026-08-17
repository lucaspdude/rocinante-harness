"use client";

// Hook that polls /api/v1/meta and returns the set of detected
// provider env vars, plus save/delete actions. The api never
// returns the values, only the booleans — the web UI uses these
// to render a "Configured / Not set" checklist in the Providers
// settings tab and in the onboarding step.
//
// Polling is 5 s by default. The status is read-only on the web
// side: the actual key write goes through api.setProviderKey /
// api.deleteProviderKey, which POSTs to
// /api/v1/providers/{name}/key on the api (chmod 0600 file on
// the api's share dir). The api then re-reads the file on every
// omp session spawn, so a new key is picked up by the next
// prompt without any process restart.

import { useCallback, useEffect, useState } from "react";
import { api } from "../api/client";

export interface ProviderStatus {
  anthropic: boolean;
  openai: boolean;
  gemini: boolean;
  openrouter: boolean;
  minimax: boolean;
}

export interface ProviderDef {
  key: keyof ProviderStatus;
  label: string;
  envVar: string;
  installHint: string;
  // helpUrl is shown as an external "where do I get this?" link
  // next to the input field. The user goes there, creates a key,
  // pastes it into the form, and saves.
  helpUrl: string;
}

export const PROVIDERS: ProviderDef[] = [
  {
    key: "anthropic",
    label: "Anthropic",
    envVar: "ANTHROPIC_API_KEY",
    installHint: "console.anthropic.com → Settings → API Keys",
    helpUrl: "https://console.anthropic.com/settings/keys",
  },
  {
    key: "openai",
    label: "OpenAI",
    envVar: "OPENAI_API_KEY",
    installHint: "platform.openai.com → API keys",
    helpUrl: "https://platform.openai.com/api-keys",
  },
  {
    key: "gemini",
    label: "Gemini",
    envVar: "GEMINI_API_KEY",
    installHint: "aistudio.google.com → API keys",
    helpUrl: "https://aistudio.google.com/apikey",
  },
  {
    key: "openrouter",
    label: "OpenRouter",
    envVar: "OPENROUTER_API_KEY",
    installHint: "openrouter.ai → Keys",
    helpUrl: "https://openrouter.ai/settings/keys",
  },
  {
    key: "minimax",
    label: "Minimax (token plan)",
    envVar: "MINIMAX_API_KEY",
    installHint: "MiniMax dashboard → Token plan → API key",
    helpUrl: "https://minimax.io/dashboard",
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
  saveKey: (name: ProviderDef["key"], key: string) => Promise<void>;
  deleteKey: (name: ProviderDef["key"]) => Promise<void>;
  saving: ProviderDef["key"] | null;
} {
  const [status, setStatus] = useState<ProviderStatus | null>(null);
  const [meta, setMeta] = useState<Omit<MetaResponse, "providers"> | null>(
    null
  );
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const [saving, setSaving] = useState<ProviderDef["key"] | null>(null);

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

  const reload = useCallback(() => setTick((n) => n + 1), []);

  const saveKey = useCallback(
    async (name: ProviderDef["key"], key: string) => {
      setSaving(name);
      try {
        await api.post(`/api/v1/providers/${name}/key`, {
          json: { key },
        });
        // Optimistic update so the form flips to "configured"
        // immediately; the next poll will re-confirm.
        setStatus((prev) =>
          prev ? { ...prev, [name]: true } : prev
        );
      } finally {
        setSaving(null);
      }
    },
    []
  );

  const deleteKey = useCallback(async (name: ProviderDef["key"]) => {
    setSaving(name);
    try {
      await api.delete(`/api/v1/providers/${name}/key`);
      setStatus((prev) =>
        prev ? { ...prev, [name]: false } : prev
      );
    } finally {
      setSaving(null);
    }
  }, []);

  return { status, meta, error, reload, saveKey, deleteKey, saving };
}
