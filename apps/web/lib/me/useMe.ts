"use client";

// useMe — fetches /api/v1/me once per page-mount and caches the
// result module-wide. The folder picker needs `home` to expand
// "~" in path inputs and to render the initial breadcrumb. We
// cache at module scope so re-mounts in the same session skip
// the network round-trip; the cache is cleared on full reload.

import { useEffect, useState } from "react";
import { api } from "../api/client";

export interface MeResponse {
  home: string;
  user: string;
  host: string;
}

let cached: MeResponse | null = null;
let inflight: Promise<MeResponse | null> | null = null;

async function fetchMe(): Promise<MeResponse | null> {
  if (cached) return cached;
  if (inflight) return inflight;
  inflight = api
    .get<MeResponse>("/api/v1/me")
    .then((res) => {
      cached = res;
      return res;
    })
    .catch(() => {
      // The api throws ApiClientError on 401/non-200. We swallow
      // it here so the picker still renders (just without ~).
      return null;
    })
    .finally(() => {
      inflight = null;
    });
  return inflight;
}

export function useMe(): { me: MeResponse | null } {
  const [me, setMe] = useState<MeResponse | null>(cached);
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
