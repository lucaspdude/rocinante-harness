"use client";

// useSshServers — hook backing the SshServerPanel.
//
// Talks to /api/v1/ssh/servers:
//
//	GET    /api/v1/ssh/servers
//	POST   /api/v1/ssh/servers                       body { label, host, port, username, key_id }
//	DELETE /api/v1/ssh/servers/{id}
//	POST   /api/v1/ssh/servers/{id}/test            returns { status, message }
//
// The server `test` endpoint (PR-05) returns a classified outcome
// — one of "ok", "auth_failed", "conn_refused", "network",
// "not_installed" — instead of raw ssh errors. The hook surfaces
// that outcome via the resolve value of `test()` so the panel can
// render the matching i18n label without re-parsing the message.
//
// Polling interval: 5s when the panel is mounted, 0 (skip) when
// the tab is hidden — mirrors the useSshKeys/useProviders pattern
// that guards against setInterval(0) runaway loops (PR-40 lesson).

import { useCallback, useEffect, useState } from "react";
import { api, ApiClientError } from "../api/client";

export interface SshServer {
  id: string;
  label: string;
  host: string;
  port: number;
  username: string;
  key_id: string;
  created_at: string;
}

// TestOutcome matches the TestOutcome constants in the api's
// internal/ssh package. Kept in sync by name; both sides fall
// back to the default locale when a key is unknown.
export type TestOutcome =
  | "ok"
  | "auth_failed"
  | "conn_refused"
  | "network"
  | "not_installed";

export interface TestResponse {
  status: TestOutcome;
  message: string;
}

interface ListResponse {
  servers: SshServer[];
}

interface CreateArgs {
  label: string;
  host: string;
  port: number;
  username: string;
  keyId: string;
}

const EMPTY: SshServer[] = [];

export function useSshServers(intervalMs = 5000): {
  servers: SshServer[];
  loading: boolean;
  error: string | null;
  reload: () => void;
  create: (args: CreateArgs) => Promise<SshServer>;
  remove: (id: string) => Promise<void>;
  test: (id: string) => Promise<TestResponse>;
} {
  const [servers, setServers] = useState<SshServer[]>(EMPTY);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await api.get<ListResponse>("/api/v1/ssh/servers");
        if (cancelled) return;
        setServers(res.servers ?? EMPTY);
        setError(null);
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    // Defensive: setInterval(fn, 0) is a tight loop in JS. Skip
    // the interval when the panel is hidden (PR-40 lesson).
    if (intervalMs <= 0) return () => { cancelled = true; };
    const id = setInterval(load, intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [intervalMs, tick]);

  const reload = useCallback(() => setTick((n) => n + 1), []);

  const create = useCallback(
    async (args: CreateArgs): Promise<SshServer> => {
      try {
        const res = await api.post<SshServer>("/api/v1/ssh/servers", {
          json: {
            label: args.label,
            host: args.host,
            port: args.port,
            username: args.username,
            key_id: args.keyId,
          },
        });
        // Refresh immediately so the card appears without waiting
        // for the next poll.
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
      // Optimistic remove so the UI is snappy.
      setServers((prev) => prev.filter((s) => s.id !== id));
      try {
        await api.delete(`/api/v1/ssh/servers/${id}`);
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

  const test = useCallback(
    async (id: string): Promise<TestResponse> => {
      try {
        const res = await api.post<TestResponse>(
          `/api/v1/ssh/servers/${id}/test`
        );
        // The api returns { status: TestOutcome, message: string }
        // — we surface it as-is. If the api errored (e.g. 404),
        // api.post rejects with ApiClientError and we wrap the
        // message in a network label so the UI stays consistent.
        return res;
      } catch (e) {
        if (e instanceof ApiClientError) {
          // Treat 404 as a transient state mismatch (the server was
          // deleted between the list poll and the test click); map
          // it to "network" so the badge renders without crashing.
          if (e.status === 404) {
            return { status: "network", message: e.body?.message ?? e.message };
          }
          // Other api errors (auth, validation) also bubble up as
          // network — the panel will refresh on the next poll and
          // the user can re-test.
          return { status: "network", message: e.body?.message ?? e.message };
        }
        return { status: "network", message: String(e) };
      }
    },
    []
  );

  return { servers, loading, error, reload, create, remove, test };
}
