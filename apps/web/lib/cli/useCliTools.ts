"use client";

// useCliTools — polling hook for the CLI tools panel.
//
// Talks to /api/v1/cli-tools/* on the api:
//
//   GET    /api/v1/cli-tools                  list supported CLIs
//   GET    /api/v1/cli-tools/{id}/status      install + auth state
//   POST   /api/v1/cli-tools/{id}/install     starts an install job (returns jobId)
//   GET    /api/v1/cli-tools/{id}/install/{jobId}/stream   SSE log stream
//   POST   /api/v1/cli-tools/{id}/login/start starts a login job (returns jobId + URL + code)
//   POST   /api/v1/cli-tools/{id}/login/{jobId}/ack        pipe user input back into the child
//
// Polling defaults to 5s. The PR-40 lesson is honoured: when
// intervalMs is 0 the panel is hidden, the hook skips the
// setInterval so we don't fire 200 req/s on /api/v1/cli-tools/*.

import { useCallback, useEffect, useState } from "react";
import { api, ApiClientError } from "../api/client";

export interface CliSpec {
  id: string;
  displayName: string;
  helpText: string;
}

export interface CliStatus {
  id: string;
  installed: boolean;
  authenticated: boolean;
  version?: string;
  account?: string;
  detail?: string;
}

export interface CliInstallStart {
  jobId: string;
  pid: number;
}

export interface CliLoginStart {
  jobId: string;
  authUrl?: string;
  authCode?: string;
}

export interface CliStreamHandlers {
  onLog: (line: string) => void;
  onStatus: (status: string) => void;
  onEnd: (status: string, exitCode: number | null) => void;
  onError: (err: Error) => void;
}

// useCliStatuses polls /api/v1/cli-tools/{id}/status for every
// id the api lists. Returns a Record<id, CliStatus> the panel
// can render directly. intervalMs defaults to 5s; 0 disables
// polling entirely (panel hidden).
export function useCliStatuses(intervalMs = 5000): {
  statuses: Record<string, CliStatus>;
  loading: boolean;
  error: string | null;
  reload: () => void;
} {
  const [statuses, setStatuses] = useState<Record<string, CliStatus>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        // Fetch the list of supported CLIs.
        const listRes = await api.get<{ clis: CliSpec[] }>("/api/v1/cli-tools");
        const ids = (listRes.clis ?? []).map((c) => c.id);
        // Fetch each status in parallel. A failure on one cli
        // doesn't poison the rest.
        const results = await Promise.all(
          ids.map(async (id) => {
            try {
              const s = await api.get<CliStatus>(`/api/v1/cli-tools/${id}/status`);
              return [id, s] as const;
            } catch (e) {
              return [
                id,
                {
                  id,
                  installed: false,
                  authenticated: false,
                  detail: e instanceof Error ? e.message : String(e),
                } as CliStatus,
              ] as const;
            }
          })
        );
        if (cancelled) return;
        const next: Record<string, CliStatus> = {};
        for (const [id, s] of results) next[id] = s;
        setStatuses(next);
        setError(null);
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    // Defensive: PR-40 — setInterval(fn, 0) is a tight loop.
    if (intervalMs <= 0) return () => { cancelled = true; };
    const id = setInterval(load, intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [intervalMs, tick]);

  const reload = useCallback(() => setTick((n) => n + 1), []);
  return { statuses, loading, error, reload };
}

// startInstall POSTs /api/v1/cli-tools/{id}/install and
// returns the jobId + pid the api hands back.
export async function startInstall(id: string): Promise<CliInstallStart> {
  try {
    const res = await api.post<CliInstallStart>(`/api/v1/cli-tools/${id}/install`);
    return res;
  } catch (e) {
    if (e instanceof ApiClientError) {
      throw new Error(e.body?.message ?? e.message);
    }
    throw e;
  }
}

// startLogin POSTs /api/v1/cli-tools/{id}/login/start and
// returns the jobId + captured authUrl/authCode (the regex
// pass typically populates them within a few hundred ms).
export async function startLogin(id: string): Promise<CliLoginStart> {
  try {
    const res = await api.post<CliLoginStart>(`/api/v1/cli-tools/${id}/login/start`);
    return res;
  } catch (e) {
    if (e instanceof ApiClientError) {
      throw new Error(e.body?.message ?? e.message);
    }
    throw e;
  }
}

// ackLogin POSTs /api/v1/cli-tools/{id}/login/{jobId}/ack
// with the user's typed code (or "\n" for gh). The api
// pipes it into the running child.
export async function ackLogin(
  id: string,
  jobId: string,
  value: string
): Promise<void> {
  try {
    await api.post(`/api/v1/cli-tools/${id}/login/${jobId}/ack`, {
      json: { value },
    });
  } catch (e) {
    if (e instanceof ApiClientError) {
      throw new Error(e.body?.message ?? e.message);
    }
    throw e;
  }
}

// openInstallStream subscribes to the install job's SSE
// stream and invokes the supplied callbacks. The returned
// function closes the EventSource.
//
// We don't use the shared SSE wrapper because the install
// stream is short-lived (<2min) and the EventSource
// semantics already match what we need: auto-reconnect on
// disconnect, text/event-stream parsing.
export function openInstallStream(
  id: string,
  jobId: string,
  handlers: CliStreamHandlers
): () => void {
  const url = `/api/v1/cli-tools/${id}/install/${jobId}/stream`;
  const es = new EventSource(url);
  es.addEventListener("log", (ev) => {
    try {
      const data = JSON.parse((ev as MessageEvent).data) as {
        line: string;
        job_id: string;
      };
      handlers.onLog(data.line);
    } catch (e) {
      handlers.onError(e instanceof Error ? e : new Error(String(e)));
    }
  });
  es.addEventListener("status", (ev) => {
    try {
      const data = JSON.parse((ev as MessageEvent).data) as {
        status: string;
      };
      handlers.onStatus(data.status);
    } catch {
      // ignore parse errors; the end event will follow.
    }
  });
  es.addEventListener("end", (ev) => {
    try {
      const data = JSON.parse((ev as MessageEvent).data) as {
        status: string;
        exit_code: number | null;
      };
      handlers.onEnd(data.status, data.exit_code);
    } catch {
      handlers.onEnd("done", null);
    } finally {
      es.close();
    }
  });
  es.onerror = () => {
    handlers.onError(new Error("install stream disconnected"));
    es.close();
  };
  return () => es.close();
}