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

  async clear(): Promise<void> {
    if (typeof window === "undefined") return;
    window.localStorage.removeItem(KEY);
  }
}

export const tokenStore = new TokenStore();
