"use client";

// Phase 7 — item 03: client child that subscribes to the
// `rh:auth:expired` window event emitted by lib/api/client.ts
// (line 54-57) when the api returns 401 with code
// `auth_token_expired`, `auth_missing`, or `auth_invalid_token`.
//
// The listener:
//   1. clears the token + cookie via tokenStore.clear(),
//   2. navigates to /<locale>/login?next=<original>.
//
// Lives in a client child (not the root layout) because the
// root layout is a server component — adding useEffect to it
// would throw at runtime.

import { useEffect } from "react";
import { useI18n, useLocalizedPath } from "../i18n";
import { tokenStore } from "./token-store";

export function AuthExpiredListener() {
  const i18n = useI18n();
  const lp = useLocalizedPath();

  useEffect(() => {
    function onExpired() {
      void tokenStore.clear();
      window.location.href = `/${i18n.locale}/login?next=${encodeURIComponent(window.location.pathname + window.location.search)}`;
    }
    window.addEventListener("rh:auth:expired", onExpired);
    return () => {
      window.removeEventListener("rh:auth:expired", onExpired);
    };
  }, [i18n.locale, lp]);

  return null;
}
