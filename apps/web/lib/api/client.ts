// API client wrapper for the rocinante-harness front.
// Reads the access token from the TokenStore and attaches it as
// a Bearer header. When the api returns 401 with code
// auth_token_expired or auth_missing, the client emits a
// `rh:auth:expired` window event so the root layout can redirect.

import { tokenStore, type StoredTokens } from "../auth/token-store";

export type AuthExpiredReason = "auth_token_expired" | "auth_missing" | "auth_invalid_token";

// Use a relative path so the browser always talks to the same origin
// (the web server on :30178). The web server proxies /api/v1/* to
// the real api on :30179 via the Next.js rewrite in next.config.ts.
// This avoids CORS issues entirely and lets the same bundle work
// for any deployment topology (local, LAN, public hostname).
const API_URL = "";

export interface ApiError {
  code: string;
  message?: string;
}

export interface ApiOptions extends RequestInit {
  json?: unknown;
  unauthenticated?: boolean;
}

export class ApiClientError extends Error {
  status: number;
  body: ApiError;
  constructor(status: number, body: ApiError) {
    super(body.code);
    this.status = status;
    this.body = body;
  }
}

export class TokenProvider {
  private cached: StoredTokens | null = null;
  async getAccess(): Promise<string | null> {
    if (!this.cached) {
      this.cached = await tokenStore.load();
    }
    return this.cached?.access_token ?? null;
  }
  async invalidate(): Promise<void> {
    this.cached = null;
    await tokenStore.clear();
  }
}

export const tokenProvider = new TokenProvider();

function emitAuthExpired(reason: AuthExpiredReason) {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new CustomEvent("rh:auth:expired", { detail: reason }));
}

export async function apiFetch<T = unknown>(
  path: string,
  opts: ApiOptions = {}
): Promise<T> {
  const headers = new Headers(opts.headers ?? {});
  let body = opts.body;
  if (opts.json !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(opts.json);
  }
  if (!opts.unauthenticated) {
    const token = await tokenProvider.getAccess();
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }
  }
  const url = path.startsWith("http") ? path : `${API_URL}${path}`;
  const init: RequestInit = { ...opts, headers, body };
  const res = await fetch(url, init);
  const text = await res.text();
  let parsed: unknown = null;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = text;
    }
  }
  if (!res.ok) {
    const errBody = (parsed as ApiError) ?? { code: "internal", message: text };
    if (
      res.status === 401 &&
      (errBody.code === "auth_token_expired" ||
        errBody.code === "auth_missing" ||
        errBody.code === "auth_invalid_token")
    ) {
      emitAuthExpired(errBody.code);
    }
    throw new ApiClientError(res.status, errBody);
  }
  return parsed as T;
}

export const api = {
  get: <T = unknown>(path: string, opts?: ApiOptions) =>
    apiFetch<T>(path, { ...opts, method: "GET" }),
  post: <T = unknown>(path: string, opts?: ApiOptions) =>
    apiFetch<T>(path, { ...opts, method: "POST" }),
  put: <T = unknown>(path: string, opts?: ApiOptions) =>
    apiFetch<T>(path, { ...opts, method: "PUT" }),
  delete: <T = unknown>(path: string, opts?: ApiOptions) =>
    apiFetch<T>(path, { ...opts, method: "DELETE" }),
};
