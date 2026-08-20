"use client";

// useDefaultModel — exposes the api's /api/v1/meta default_model
// for the model picker (PR-02). Polls meta with a 60s cadence to
// pick up OMP_DEFAULT_MODEL changes without a full reload.
//
// Implementation note: we don't piggy-back on useProviders because
// the model picker is the only consumer and is often rendered on
// pages that don't otherwise care about provider state. Polling meta
// directly keeps the dep graph tight.

import { useEffect, useState } from "react";
import { api } from "../api/client";

interface MetaResponse {
  api_version: string;
  omp_version: string;
  protocol_version: number;
  omp_bin: string;
  providers: unknown[];
  default_model?: string;
}

export function useDefaultModel(intervalMs = 60_000): {
  defaultModel: string;
  loading: boolean;
} {
  const [defaultModel, setDefaultModel] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await api.get<MetaResponse>("/api/v1/meta", {
          unauthenticated: true,
        });
        if (cancelled) return;
        setDefaultModel(res?.default_model ?? "");
      } catch {
        // Meta is best-effort. Leave the cached value on error.
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
  }, [intervalMs]);

  return { defaultModel, loading };
}