// Shared auth-status query.
//
// The harness has three distinct auth states the web needs to
// distinguish:
//
//   initialized     — api has an Ed25519 key on disk (passphrase auth is enabled).
//   auth_required   — == initialized on the api side; web reads this as the
//                     "is login enforced?" flag (when false, the api is in
//                     onboarding mode and login is not a valid flow).
//   device_known    — browser has the `rh-device-id` cookie from a prior
//                     successful sign-in; web uses this to decide between
//                     the "Sign in" CTA (first visit) and a redirect to
//                     /login (returning user).
//
// Item 01 (unauthed home redirect) defines this module. Item 03
// (settings + agent layout auth gate) imports `useAuthStatus` from
// here. Per 03-sequencing.md the contract is locked: do not add
// new exports to this file in later items.

import { useEffect, useState } from "react";
import { api, ApiClientError } from "../api/client";

export interface AuthStatus {
  initialized: boolean;
  auth_required: boolean; // == initialized on the api side
  device_known: boolean;
}

// Safe default returned by `fetchAuthStatus` on any error. Treats
// the visitor as a "first visit" (initialized=true, device_known=false)
// so the home page renders the "Sign in" CTA instead of redirecting.
const SAFE_DEFAULT: AuthStatus = {
  initialized: true,
  auth_required: true,
  device_known: false,
};

// One-shot fetcher. Returns the safe default on any error
// (network, 5xx, non-JSON) so the caller treats the visitor as a
// "first visit" (CTA, no redirect). The api endpoint is public
// (no bearer required), so `unauthenticated: true` is passed.
export async function fetchAuthStatus(): Promise<AuthStatus> {
  try {
    const res = await api.get<AuthStatus>("/api/v1/auth/status", {
      unauthenticated: true,
    });
    return {
      initialized: !!res?.initialized,
      auth_required: !!res?.auth_required,
      device_known: !!res?.device_known,
    };
  } catch (e: unknown) {
    if (e instanceof ApiClientError && e.status === 503) {
      // api explicitly returned 503 (share dir unreadable, etc.):
      // fall through to safe default.
    }
    return SAFE_DEFAULT;
  }
}

interface UseAuthStatusResult {
  loading: boolean;
  status: AuthStatus | null;
}

// React hook. Same logic + caches for the hook's lifetime (no
// re-fetch on re-render). On error, status === null.
export function useAuthStatus(): UseAuthStatusResult {
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<AuthStatus | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetchAuthStatus()
      .then((s) => {
        if (cancelled) return;
        setStatus(s);
        setLoading(false);
      })
      .catch(() => {
        if (cancelled) return;
        setStatus(SAFE_DEFAULT);
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return { loading, status };
}
