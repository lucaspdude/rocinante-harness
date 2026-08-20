"use client";

// Hook for /api/v1/models/catalog. PR-02 spec: 1h TTL on the
// server; the web side caches per-render and revalidates on every
// poll. For now we just fetch when the search query changes.
//
// PR-11: pass the user's locale to the catalog endpoint so the
// server can attach per-locale price fields (cost_input_local,
// cost_output_local, currency) to each entry.

import { useEffect, useState } from "react";
import { api } from "../api/client";
import type { Locale } from "../i18n/schema";

export type AuthKind = "paste-key" | "oauth" | "keyless";

export interface ModelEntry {
  id: string;
  provider: string;
  name: string;
  context_length?: number;
  max_tokens?: number;
  modalities?: string[];
  cost_input?: number;
  cost_output?: number;
  cost_input_local?: number;
  cost_output_local?: number;
  currency?: string;
  cost_cache_read?: number;
  cost_cache_write?: number;
  reasoning?: boolean;
  thinking_supported?: boolean;
  auth_supported?: boolean;
  source?: string;
  selectable: boolean;
  stale?: boolean;
}

interface CatalogResponse {
  results: ModelEntry[];
  count: number;
  stale: boolean;
}

// Module-scope cache, same shape as useMe: the catalog is keyed by
// locale+query and is stable for the lifetime of the page, so a
// re-opened picker paints the previous results instantly instead of
// showing an empty dropdown for the debounce + round-trip window.
const cached = new Map<string, CatalogResponse>();
const inflight = new Map<string, Promise<CatalogResponse>>();

function catalogKey(query: string, locale: Locale): string {
  return `${locale}:${query}`;
}

async function fetchCatalog(
  query: string,
  locale: Locale
): Promise<CatalogResponse> {
  const key = catalogKey(query, locale);
  const hit = cached.get(key);
  if (hit) return hit;
  const pending = inflight.get(key);
  if (pending) return pending;
  const promise = api
    .get<CatalogResponse>(
      `/api/v1/models/catalog?q=${encodeURIComponent(query)}&selectable=true&limit=10&locale=${encodeURIComponent(locale)}`,
      { unauthenticated: true }
    )
    .then((res) => {
      cached.set(key, res);
      return res;
    })
    .finally(() => {
      inflight.delete(key);
    });
  inflight.set(key, promise);
  return promise;
}

export function useModelCatalog(
  query: string,
  locale: Locale = "en-US",
  debounceMs = 200
): {
  models: ModelEntry[];
  loading: boolean;
  error: string | null;
  stale: boolean;
} {
  const seed = cached.get(catalogKey(query, locale));
  const [models, setModels] = useState<ModelEntry[]>(seed?.results ?? []);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [stale, setStale] = useState<boolean>(!!seed?.stale);

  useEffect(() => {
    let cancelled = false;
    const key = catalogKey(query, locale);
    const hit = cached.get(key);
    if (hit) {
      setModels(hit.results ?? []);
      setStale(!!hit.stale);
      setError(null);
      return;
    }
    const handle = setTimeout(async () => {
      setLoading(true);
      try {
        const res = await fetchCatalog(query, locale);
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
  }, [query, locale, debounceMs]);

  return { models, loading, error, stale };
}