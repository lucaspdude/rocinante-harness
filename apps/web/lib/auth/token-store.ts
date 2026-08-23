// Token store backed by localStorage.
//
// The earlier implementation used IndexedDB + WebCrypto PBKDF2/AES-GCM
// to encrypt tokens at rest. That required a secure context
// (https / localhost secure / 127.0.0.1 secure) for crypto.subtle,
// and IndexedDB is async — together they introduced flakes in both
// the dev tools and headless browsers (chromium runs E2E over plain
// http, which is NOT a secure context for crypto.subtle).
//
// This module stores the access + refresh tokens in plain
// localStorage. The api already binds the access token's trust to
// the encrypted Ed25519 key on the server, so an attacker who
// steals localStorage still has to log in. For the MVP this is
// acceptable; hardening can layer an opaque at-rest key on top
// later without changing the call sites.

export interface StoredTokens {
  access_token: string;
  refresh_token: string;
  device_id: string;
}

const KEY = "rh-tokens-v1";

export class TokenStore {
  async save(tokens: StoredTokens): Promise<void> {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(KEY, JSON.stringify(tokens));
    // Phase 7 — item 01: mirror the device_id to a long-lived
    // cookie so the api can read it server-side via the new
    // /api/v1/auth/status endpoint. The web uses this to
    // distinguish "first visit" (no cookie) from "returning
    // user" (cookie present) on the home route.
    // `secure` is conditional: the live harness at
    // http://192.168.0.222:30178 is NOT a secure context;
    // setting `secure` unconditionally would silently drop
    // the cookie. The i18n locale cookie at
    // lib/i18n/index.tsx is the precedent.
    const secure = window.isSecureContext ? "; secure" : "";
    document.cookie = `rh-device-id=${encodeURIComponent(tokens.device_id)}; path=/; max-age=31536000; samesite=lax${secure}`;
  }

  async load(): Promise<StoredTokens | null> {
    if (typeof window === "undefined") return null;
    const raw = window.localStorage.getItem(KEY);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as StoredTokens;
    } catch {
      return null;
    }
  }

  // Synchronous peek — useful for SSR-friendly checks (does the
  // user have any token at all?). Returns false during SSR.
  peek(): boolean {
    if (typeof window === "undefined") return false;
    return window.localStorage.getItem(KEY) !== null;
  }

  async clear(): Promise<void> {
    if (typeof window === "undefined") return;
    window.localStorage.removeItem(KEY);
    // Phase 7 — item 01: also clear the rh-device-id cookie so a
    // subsequent visit reverts to "first visit" semantics. Setting
    // max-age=0 (or past expiry) is the standard browser-side
    // removal technique.
    document.cookie = "rh-device-id=; path=/; max-age=0; samesite=lax";
  }
}

export const tokenStore = new TokenStore();
