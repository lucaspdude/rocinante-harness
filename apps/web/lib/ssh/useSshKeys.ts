"use client";

// useSshKeys — hook backing the GitSshPanel.
//
// Talks to /api/v1/ssh/keys:
//   GET    /api/v1/ssh/keys
//   POST   /api/v1/ssh/keys         body { label, provider } (provider optional)
//   DELETE /api/v1/ssh/keys/{id}
//
// The POST response includes a one-shot `private_key` field — the
// hook surfaces that to the caller via the resolve value of
// `generate()`. The component is responsible for showing the key
// to the user exactly once and warning that it will not be re-sent.
//
// Polling interval: 5s when the panel is mounted, 0 (skip) when the
// tab is hidden — mirrors the useProjects/useProviders pattern that
// guards against setInterval(0) runaway loops (see PR-40 lesson).

import { useCallback, useEffect, useState } from "react";
import { api, ApiClientError } from "../api/client";

export interface SshKey {
  id: string;
  label: string;
  provider: string;
  fingerprint: string;
  public_key: string;
  created_at: string;
}

export interface SshGenerateResult {
  key: SshKey;
  private_key: string;
}

interface ListResponse {
  keys: SshKey[];
}

const EMPTY: SshKey[] = [];

export function useSshKeys(intervalMs = 5000): {
  keys: SshKey[];
  loading: boolean;
  error: string | null;
  reload: () => void;
  generate: (label: string, provider: string) => Promise<SshGenerateResult>;
  remove: (id: string) => Promise<void>;
} {
  const [keys, setKeys] = useState<SshKey[]>(EMPTY);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await api.get<ListResponse>("/api/v1/ssh/keys");
        if (cancelled) return;
        setKeys(res.keys ?? EMPTY);
        setError(null);
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    if (intervalMs <= 0) return () => { cancelled = true; };
    const id = setInterval(load, intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [intervalMs, tick]);

  const reload = useCallback(() => setTick((n) => n + 1), []);

  const generate = useCallback(
    async (label: string, provider: string): Promise<SshGenerateResult> => {
      try {
        const res = await api.post<SshGenerateResult>(
          "/api/v1/ssh/keys",
          { json: { label, provider } }
        );
        // Refresh the list immediately so the new card appears
        // without waiting for the next poll.
        reload();
        return res;
      } catch (e) {
        if (e instanceof ApiClientError) {
          throw new Error(e.body?.message ?? e.message);
        }
        throw e;
      }
    },
    [reload]
  );

  const remove = useCallback(
    async (id: string): Promise<void> => {
      // Optimistic remove so the UI is snappy; reload picks up any
      // api-side cleanup (file/config block) on the next tick.
      setKeys((prev) => prev.filter((k) => k.id !== id));
      try {
        await api.delete(`/api/v1/ssh/keys/${id}`);
      } catch (e) {
        reload();
        if (e instanceof ApiClientError) {
          throw new Error(e.body?.message ?? e.message);
        }
        throw e;
      }
    },
    [reload]
  );

  return { keys, loading, error, reload, generate, remove };
}
