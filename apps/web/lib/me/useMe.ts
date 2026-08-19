"use client";

// useMe — fetches /api/v1/me once per page-mount and caches the
// result module-wide. The folder picker needs `home` to expand
// "~" in path inputs and to render the initial breadcrumb. We
// cache at module scope so re-mounts in the same session skip
// the network round-trip; the cache is cleared on full reload.
//
// PR-03: when the api is unreachable (no token, 401, network),
// resolve to a sane default so the picker can still render and
// fall back to "/" / "/root" instead of staying null forever.

import { useEffect, useState } from "react";
import { api } from "../api/client";

export interface MeResponse {
  home: string;
  user: string;
  host: string;
}

// Default used when the api is unreachable. The api also falls
// back to /root server-side when $HOME is unset, so the two
// sides agree.
const FALLBACK_HOME = "/root";
const FALLBACK_USER = "anonymous";

function fallbackMe(): MeResponse {
  return {
    home: FALLBACK_HOME,
    user: FALLBACK_USER,
    host:
      typeof window !== "undefined" && window.location
        ? window.location.host
        : "",
  };
}

let cached: MeResponse | null = null;
let inflight: Promise<MeResponse> | null = null;

async function fetchMe(): Promise<MeResponse> {
  if (cached) return cached;
  if (inflight) return inflight;
  inflight = api
    .get<MeResponse>("/api/v1/me")
    .then((res) => {
      cached = res;
      return res;
    })
    .catch(() => {
      // The api throws ApiClientError on 401/non-200/network.
      // Return the fallback so the picker still resolves with a
      // usable home directory instead of staying null forever.
      // The picker shows "/" by default (D1) until the user
      // signs in; once useMe re-runs with a real token, the
      // real home replaces the fallback.
      return fallbackMe();
    })
    .finally(() => {
      inflight = null;
    });
  return inflight;
}

export function useMe(): { me: MeResponse } {
  const [me, setMe] = useState<MeResponse>(cached ?? fallbackMe());
  useEffect(() => {
    if (cached) return;
    let cancelled = false;
    fetchMe().then((res) => {
      if (!cancelled) setMe(res);
    });
    return () => {
      cancelled = true;
    };
  }, []);
  return { me };
}