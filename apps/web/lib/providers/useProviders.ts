"use client";

// Hook that polls /api/v1/meta and returns the set of detected
// providers (PR-01 reshape + post-review capabilities).
//
// The api never returns the values, only the booleans — the web UI
// uses these to render a "Configured / Not set" checklist in the
// Providers settings tab and in the onboarding step.
//
// Polling is 5 s by default. The status is read-only on the web
// side: the actual key write goes through api.setProviderKey /
// api.deleteProviderKey, which POSTs to
// /api/v1/providers/{name}/key on the api (chmod 0600 file on
// the api's share dir). The api then re-reads the file on every
// omp session spawn, so a new key is picked up by the next
// prompt without any process restart.
//
// PR-01 also adds /api/v1/login/providers, which exposes the same
// provider list (cached 5s server-side). This hook reads /meta
// for backwards compat; new code should prefer useLoginProviders.

import { useCallback, useEffect, useState } from "react";
import { api } from "../api/client";

export interface ProviderInfo {
  id: string;
  name: string;
  available: boolean;
  authenticated: boolean;
  // Capabilities (orthogonal, per PR-01 advisory 4):
  env_vars?: string[];
  supports_login: boolean;
  keyless: boolean;
  help_url?: string;
}

interface MetaResponse {
  api_version: string;
  omp_version: string;
  protocol_version: number;
  omp_bin: string;
  providers: ProviderInfo[];
}

interface LoginProvidersResponse {
  providers: ProviderInfo[];
  cached_at: string;
}

// Legacy single-env-var convenience: return the first env var or
// undefined. Used by the panel's save form which currently writes
// only one key.
export function envVarOf(p: ProviderInfo): string | undefined {
  return p.env_vars?.[0];
}

export function useProviders(intervalMs = 5000): {
  providers: ProviderInfo[];
  meta: Omit<MetaResponse, "providers"> | null;
  error: string | null;
  reload: () => void;
  saveKey: (name: string, key: string) => Promise<void>;
  deleteKey: (name: string) => Promise<void>;
  saving: string | null;
} {
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [meta, setMeta] = useState<Omit<MetaResponse, "providers"> | null>(
    null
  );
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const [saving, setSaving] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await api.get<MetaResponse>("/api/v1/meta");
        if (cancelled) return;
        setProviders(res.providers ?? []);
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

  const saveKey = useCallback(async (name: string, key: string) => {
    setSaving(name);
    try {
      await api.post(`/api/v1/providers/${name}/key`, { json: { key } });
      setProviders((prev) =>
        prev.map((p) => (p.id === name ? { ...p, authenticated: true } : p))
      );
    } finally {
      setSaving(null);
    }
  }, []);

  const deleteKey = useCallback(async (name: string) => {
    setSaving(name);
    try {
      await api.delete(`/api/v1/providers/${name}/key`);
      setProviders((prev) =>
        prev.map((p) => (p.id === name ? { ...p, authenticated: false } : p))
      );
    } finally {
      setSaving(null);
    }
  }, []);

  return { providers, meta, error, reload, saveKey, deleteKey, saving };
}

// useLoginProviders polls the dedicated login providers endpoint
// (cache 5s server-side). Same shape as useProviders minus meta.
export function useLoginProviders(intervalMs = 5000): {
  providers: ProviderInfo[];
  reload: () => void;
  error: string | null;
} {
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await api.get<LoginProvidersResponse>(
          "/api/v1/login/providers",
          { unauthenticated: true }
        );
        if (cancelled) return;
        setProviders(res.providers ?? []);
        setError(null);
      } catch (e) {
        if (!cancelled) {
          const err = e as { body?: { message?: string }; message?: string };
          setError(err.body?.message ?? err.message ?? "failed");
        }
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
  return { providers, reload, error };
}
