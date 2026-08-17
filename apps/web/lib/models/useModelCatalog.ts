"use client";

// Hook for /api/v1/models/catalog. PR-02 spec: 1h TTL on the
// server; the web side caches per-render and revalidates on every
// poll. For now we just fetch when the search query changes.

import { useEffect, useState } from "react";
import { api } from "../api/client";

export type AuthKind = "paste-key" | "oauth" | "keyless";

export interface ModelEntry {
  id: string;
  provider: string;
  name: string;
  context_length?: number;
  modalities?: string[];
  cost_input?: number;
  cost_output?: number;
  selectable: boolean;
  stale?: boolean;
}

interface CatalogResponse {
  results: ModelEntry[];
  count: number;
  stale: boolean;
}

export function useModelCatalog(query: string, debounceMs = 200): {
  models: ModelEntry[];
  loading: boolean;
  error: string | null;
  stale: boolean;
} {
  const [models, setModels] = useState<ModelEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [stale, setStale] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const handle = setTimeout(async () => {
      setLoading(true);
      try {
        const res = await api.get<CatalogResponse>(
          `/api/v1/models/catalog?q=${encodeURIComponent(query)}&selectable=true&limit=10`,
          { unauthenticated: true }
        );
        if (cancelled) return;
        setModels(res.results ?? []);
        setStale(!!res.stale);
        setError(null);
      } catch (e: unknown) {
        const err = e as { body?: { message?: string }; message?: string };
        if (!cancelled) setError(err.body?.message ?? err.message ?? "failed");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }, debounceMs);
    return () => {
      cancelled = true;
      clearTimeout(handle);
    };
  }, [query, debounceMs]);

  return { models, loading, error, stale };
}
