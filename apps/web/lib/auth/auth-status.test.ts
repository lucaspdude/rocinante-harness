// Phase 7 — item 01 unit tests for the shared auth-status module.
//
// The four cases below exercise the contract that item 03 also
// depends on (per 03-sequencing.md). Keep them stable.

import { describe, it, expect, afterEach, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { fetchAuthStatus, useAuthStatus } from "./auth-status";

interface FetchAuthStatusOptions {
  ok: boolean;
  status: number;
  body: unknown;
}

function stubFetch({ ok, status, body }: FetchAuthStatusOptions) {
  const fetchMock = vi.fn().mockResolvedValue({
    ok,
    status,
    text: () => Promise.resolve(JSON.stringify(body)),
    json: () => Promise.resolve(body),
  });
  globalThis.fetch = fetchMock as unknown as typeof fetch;
  return fetchMock;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("fetchAuthStatus", () => {
  it("happy path returns the parsed body", async () => {
    stubFetch({
      ok: true,
      status: 200,
      body: { initialized: true, auth_required: true, device_known: true },
    });
    const result = await fetchAuthStatus();
    expect(result).toEqual({
      initialized: true,
      auth_required: true,
      device_known: true,
    });
  });

  it("503 returns the safe default", async () => {
    stubFetch({
      ok: false,
      status: 503,
      body: { code: "internal", message: "share dir unreadable" },
    });
    const result = await fetchAuthStatus();
    expect(result).toEqual({
      initialized: true,
      auth_required: true,
      device_known: false,
    });
  });

  it("network error returns the safe default", async () => {
    globalThis.fetch = vi
      .fn()
      .mockRejectedValue(new Error("network down")) as unknown as typeof fetch;
    const result = await fetchAuthStatus();
    expect(result).toEqual({
      initialized: true,
      auth_required: true,
      device_known: false,
    });
  });
});

describe("useAuthStatus", () => {
  it("triggers exactly one fetch on mount, then caches for the hook's lifetime", async () => {
    const fetchMock = stubFetch({
      ok: true,
      status: 200,
      body: { initialized: true, auth_required: true, device_known: true },
    });
    const { result, rerender } = renderHook(() => useAuthStatus());
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.status).toEqual({
      initialized: true,
      auth_required: true,
      device_known: true,
    });
    const callsAfterMount = fetchMock.mock.calls.length;
    // Re-render 3 times; the empty useEffect dep array
    // guarantees no re-fetch.
    rerender();
    rerender();
    rerender();
    expect(fetchMock.mock.calls.length).toBe(callsAfterMount);
    expect(callsAfterMount).toBe(1);
  });
});
