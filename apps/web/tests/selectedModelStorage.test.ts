// PR-3: selected model persistence — unit tests for the
// SSR-safe localStorage helpers used by the composer.
//
// The helpers guard `typeof window === "undefined"` so they can be
// imported from a Server Component without throwing; the test
// exercises that contract as well as the happy path.

import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from "vitest";
import {
  readSelectedModelId,
  writeSelectedModelId,
  SELECTED_MODEL_STORAGE_KEY,
} from "../lib/agent/selectedModelStorage";

describe("selectedModelStorage", () => {
  let store: Record<string, string>;
  let getItem: Mock<(k: string) => string | null>;
  let setItem: Mock<(k: string, v: string) => void>;

  beforeEach(() => {
    store = {};
    getItem = vi.fn((k: string) => store[k] ?? null);
    setItem = vi.fn((k: string, v: string) => {
      store[k] = v;
    });
    vi.stubGlobal("window", { localStorage: { getItem, setItem } });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("writes through to window.localStorage under the rh key", () => {
    writeSelectedModelId("MiniMax-M3");
    expect(setItem).toHaveBeenCalledWith(SELECTED_MODEL_STORAGE_KEY, "MiniMax-M3");
  });

  it("reads the value back under the same key", () => {
    store[SELECTED_MODEL_STORAGE_KEY] = "claude-sonnet-4";
    expect(readSelectedModelId()).toBe("claude-sonnet-4");
    expect(getItem).toHaveBeenCalledWith(SELECTED_MODEL_STORAGE_KEY);
  });

  it("returns null when the key has never been written", () => {
    expect(readSelectedModelId()).toBeNull();
  });

  it("returns null and does not throw on the server (no window)", () => {
    vi.unstubAllGlobals();
    expect(readSelectedModelId()).toBeNull();
    expect(() => writeSelectedModelId("MiniMax-M3")).not.toThrow();
  });

  it("swallows read errors from a misbehaving storage backend", () => {
    vi.stubGlobal("window", {
      localStorage: {
        getItem: () => {
          throw new Error("security");
        },
        setItem: () => {},
      },
    });
    expect(readSelectedModelId()).toBeNull();
  });

  it("swallows write errors (quota / disabled storage)", () => {
    vi.stubGlobal("window", {
      localStorage: {
        getItem: () => null,
        setItem: () => {
          throw new Error("quota");
        },
      },
    });
    expect(() => writeSelectedModelId("MiniMax-M3")).not.toThrow();
  });
});
