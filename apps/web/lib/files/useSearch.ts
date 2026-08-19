"use client";

// useSearch — debounced wrapper around POST /api/v1/search. PR-06.
//
// Review-followup pattern (v1.0.2 setInterval(0) fix): this hook
// uses setTimeout (debounce) only — never setInterval — so there
// is no tight-loop hazard when the consumer passes an
// intervalMs <= 0 or simply relies on the debounce alone. Each
// keystroke schedules a fresh setTimeout, cancelling the prior
// pending one via clearTimeout.

import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiClientError } from "../api/client";

export interface SearchMatch {
  path: string;
  line: number;
  column: number;
  match: string;
}

export interface SearchOptions {
  regex: boolean;
  maxResults: number;
  caseSensitive: boolean;
  fileGlob: string;
}

export interface SearchResult {
  results: SearchMatch[];
  partial: boolean;
}

export interface UseSearchState {
  results: SearchMatch[];
  partial: boolean;
  loading: boolean;
  error: string | null;
  code: string | null;
}

interface SearchRequestBody {
  root: string;
  pattern: string;
  options: SearchOptions;
}

const DEFAULT_DEBOUNCE_MS = 300;

async function runSearch(body: SearchRequestBody): Promise<SearchResult> {
  return api.post<SearchResult>("/api/v1/search", { json: body });
}

export function useSearch(
  root: string,
  pattern: string,
  options: SearchOptions,
  debounceMs: number = DEFAULT_DEBOUNCE_MS
): UseSearchState {
  const [results, setResults] = useState<SearchMatch[]>([]);
  const [partial, setPartial] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [code, setCode] = useState<string | null>(null);
  const lastRequestId = useRef(0);

  // Stabilize the options so the effect deps don't fire on every
  // parent re-render (the caller passes a fresh object each
  // render otherwise).
  const optionsKey = JSON.stringify(options);

  const doSearch = useCallback(async () => {
    if (!root || !pattern) {
      setResults([]);
      setPartial(false);
      setError(null);
      setCode(null);
      return;
    }
    const myRequestId = ++lastRequestId.current;
    setLoading(true);
    setError(null);
    setCode(null);
    try {
      const parsedOpts = JSON.parse(optionsKey) as SearchOptions;
      const res = await runSearch({ root, pattern, options: parsedOpts });
      if (myRequestId !== lastRequestId.current) return;
      setResults(res.results ?? []);
      setPartial(Boolean(res.partial));
    } catch (e: unknown) {
      if (myRequestId !== lastRequestId.current) return;
      let message = "search failed";
      let errCode: string | null = null;
      if (e instanceof ApiClientError) {
        message = e.body?.message ?? e.message ?? "search failed";
        errCode = e.body?.code ?? null;
      } else if (e instanceof Error) {
        message = e.message;
      }
      setResults([]);
      setPartial(false);
      setError(message);
      setCode(errCode);
    } finally {
      if (myRequestId === lastRequestId.current) setLoading(false);
    }
  }, [root, pattern, optionsKey]);

  useEffect(() => {
    if (!root || !pattern) {
      setResults([]);
      setPartial(false);
      setLoading(false);
      setError(null);
      setCode(null);
      return;
    }
    const handle = setTimeout(() => {
      void doSearch();
    }, debounceMs);
    return () => clearTimeout(handle);
  }, [doSearch, debounceMs, root, pattern]);

  return { results, partial, loading, error, code };
}
