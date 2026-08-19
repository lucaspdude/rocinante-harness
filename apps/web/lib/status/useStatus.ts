"use client";

// useStatus — drives the footer status pill. Polls
// /api/v1/healthz every 30s and /api/v1/meta every 60s. On health
// failure retry after 5s; 3 consecutive failures → 'fail' (red,
// "unreachable") and pause polling for 60s. Reads
// localStorage 'rh:status-meta' on mount for instant display and
// re-validates.
//
// Polling guards (intervalMs = 0 → run once, never re-arm) match
// the v1.0.2 lesson that bit us in useProjects / useFiles: never
// pass 0 to setInterval.

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "../api/client";

const HEALTH_INTERVAL_MS = 30_000;
const META_INTERVAL_MS = 60_000;
const HEALTH_RETRY_MS = 5_000;
const UNREACHABLE_PAUSE_MS = 60_000;
const META_FAILURE_PAUSE_MS = 60_000;
const HEALTH_FAILURE_THRESHOLD = 3;
const STORAGE_KEY = "rh:status-meta";

type TimerId = ReturnType<typeof setTimeout>;
type TimerMap = Partial<Record<"meta" | "health" | "retry" | "paused", TimerId>>;

export type StatusKind = "ok" | "partial" | "fail" | "loading";

export interface StatusMeta {
  api_version: string;
  omp_version: string;
}

export interface StatusState {
  kind: StatusKind;
  apiVersion: string;
  ompVersion: string;
  lastOkAt: number | null;
  lastFailAt: number | null;
  lastError: string | null;
  recheck: () => void;
}

interface CachedMeta {
  apiVersion: string;
  ompVersion: string;
  cachedAt: number;
}

function readCachedMeta(): CachedMeta | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<CachedMeta>;
    if (typeof parsed.apiVersion !== "string") return null;
    if (typeof parsed.ompVersion !== "string") return null;
    if (typeof parsed.cachedAt !== "number") return null;
    return {
      apiVersion: parsed.apiVersion,
      ompVersion: parsed.ompVersion,
      cachedAt: parsed.cachedAt,
    };
  } catch {
    return null;
  }
}

function writeCachedMeta(meta: CachedMeta): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(meta));
  } catch {
    // localStorage may be unavailable (private mode, quota); the
    // pill still works in-memory, just won't survive a reload.
  }
}

export function classify(apiV: string, ompV: string): StatusKind {
  if (apiV && ompV && apiV === ompV) return "ok";
  if (apiV && ompV) return "partial";
  if (apiV) return "partial";
  return "loading";
}

function errorMessage(e: unknown): string {
  const err = e as { body?: { message?: string }; message?: string };
  return err.body?.message ?? err.message ?? "network";
}

export function useStatus(): StatusState {
  const cached = readCachedMeta();
  const [apiVersion, setApiVersion] = useState<string>(cached?.apiVersion ?? "");
  const [ompVersion, setOmpVersion] = useState<string>(cached?.ompVersion ?? "");
  const [lastOkAt, setLastOkAt] = useState<number | null>(cached?.cachedAt ?? null);
  const [lastFailAt, setLastFailAt] = useState<number | null>(null);
  const [lastError, setLastError] = useState<string | null>(null);
  const [kind, setKind] = useState<StatusKind>(cached ? "ok" : "loading");
  const [recheckToken, setRecheckToken] = useState(0);

  const cancelledRef = useRef(false);
  const healthFailuresRef = useRef(0);
  const timersRef = useRef<TimerMap>({});
  const metaFailuresRef = useRef(0);

  const recheck = useCallback(() => {
    setRecheckToken((n) => n + 1);
  }, []);

  useEffect(() => {
    cancelledRef.current = false;
    healthFailuresRef.current = 0;
    metaFailuresRef.current = 0;
    timersRef.current = {};

    const clearTimers = () => {
      clearTimeout(timersRef.current.meta);
      clearTimeout(timersRef.current.health);
      clearTimeout(timersRef.current.retry);
      clearTimeout(timersRef.current.paused);
      timersRef.current = {};
    };

    async function checkHealth(): Promise<boolean> {
      try {
        const res = await api.get<{ ok?: boolean }>("/api/v1/healthz", {
          unauthenticated: true,
        });
        if (cancelledRef.current) return false;
        if (res?.ok === true) {
          healthFailuresRef.current = 0;
          return true;
        }
        healthFailuresRef.current += 1;
        return false;
      } catch (e: unknown) {
        if (cancelledRef.current) return false;
        healthFailuresRef.current += 1;
        setLastError(errorMessage(e));
        return false;
      }
    }

    async function fetchMeta(): Promise<StatusMeta | null> {
      try {
        const res = await api.get<StatusMeta>("/api/v1/meta", {
          unauthenticated: true,
        });
        if (cancelledRef.current) return null;
        metaFailuresRef.current = 0;
        return res;
      } catch (e: unknown) {
        if (cancelledRef.current) return null;
        metaFailuresRef.current += 1;
        setLastError(errorMessage(e));
        return null;
      }
    }

    function enterUnreachable() {
      setKind("fail");
      timersRef.current.paused = setTimeout(() => {
        if (cancelledRef.current) return;
        healthFailuresRef.current = 0;
        scheduleHealth();
      }, UNREACHABLE_PAUSE_MS);
    }

    function scheduleHealth() {
      if (cancelledRef.current) return;
      timersRef.current.health = setTimeout(async () => {
        if (cancelledRef.current) return;
        const ok = await checkHealth();
        if (cancelledRef.current) return;
        if (ok) {
          setLastOkAt(Date.now());
          setLastError(null);
          scheduleHealth();
          return;
        }
        setLastFailAt(Date.now());
        if (healthFailuresRef.current >= HEALTH_FAILURE_THRESHOLD) {
          enterUnreachable();
          return;
        }
        // Quick retry after 5s.
        timersRef.current.retry = setTimeout(async () => {
          if (cancelledRef.current) return;
          const nextOk = await checkHealth();
          if (cancelledRef.current) return;
          if (nextOk) {
            setLastOkAt(Date.now());
            setLastError(null);
            scheduleHealth();
            return;
          }
          setLastFailAt(Date.now());
          if (healthFailuresRef.current >= HEALTH_FAILURE_THRESHOLD) {
            enterUnreachable();
          } else {
            scheduleHealth();
          }
        }, HEALTH_RETRY_MS);
      }, HEALTH_INTERVAL_MS);
    }

    function scheduleMeta() {
      if (cancelledRef.current) return;
      timersRef.current.meta = setTimeout(async () => {
        if (cancelledRef.current) return;
        const res = await fetchMeta();
        if (cancelledRef.current) return;
        if (res) {
          setApiVersion(res.api_version ?? "");
          setOmpVersion(res.omp_version ?? "");
          setKind((prev) => classify(res.api_version ?? "", res.omp_version ?? ""));
          writeCachedMeta({
            apiVersion: res.api_version ?? "",
            ompVersion: res.omp_version ?? "",
            cachedAt: Date.now(),
          });
          scheduleMeta();
          return;
        }
        // Meta is heavier; back off for 60s on failure.
        timersRef.current.meta = setTimeout(scheduleMeta, META_FAILURE_PAUSE_MS);
      }, META_INTERVAL_MS);
    }

    async function bootstrap() {
      const [healthOk, meta] = await Promise.all([checkHealth(), fetchMeta()]);
      if (cancelledRef.current) return;
      if (meta) {
        setApiVersion(meta.api_version ?? "");
        setOmpVersion(meta.omp_version ?? "");
        writeCachedMeta({
          apiVersion: meta.api_version ?? "",
          ompVersion: meta.omp_version ?? "",
          cachedAt: Date.now(),
        });
      }
      if (healthOk) {
        setLastOkAt(Date.now());
        setLastError(null);
        setKind(classify(meta?.api_version ?? "", meta?.omp_version ?? ""));
      } else {
        setLastFailAt(Date.now());
        if (healthFailuresRef.current >= HEALTH_FAILURE_THRESHOLD) {
          enterUnreachable();
        } else {
          const retryOnce = () => {
            timersRef.current.retry = setTimeout(async () => {
              if (cancelledRef.current) return;
              const ok = await checkHealth();
              if (cancelledRef.current) return;
              if (ok) {
                setLastOkAt(Date.now());
                setLastError(null);
                setKind((cur) => (cur === "fail" ? "ok" : cur));
                scheduleHealth();
                return;
              }
              setLastFailAt(Date.now());
              if (healthFailuresRef.current >= HEALTH_FAILURE_THRESHOLD) {
                enterUnreachable();
              } else {
                retryOnce();
              }
            }, HEALTH_RETRY_MS);
          };
          retryOnce();
        }
      }
      scheduleMeta();
    }

    bootstrap();

    return () => {
      cancelledRef.current = true;
      clearTimers();
    };
  }, [recheckToken]);

  return {
    kind,
    apiVersion,
    ompVersion,
    lastOkAt,
    lastFailAt,
    lastError,
    recheck,
  };
}
