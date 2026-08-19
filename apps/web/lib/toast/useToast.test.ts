// Unit tests for the pure toast-store helpers. React Context
// components are exercised manually; these tests cover the
// behavior the user can see — adding, capping at 5, dismissing,
// default durations, and the error → code extraction.

import { describe, it, expect } from "vitest";
import {
  addToast,
  DEFAULT_DURATIONS_MS,
  dismissToast,
  extractError,
  nextToastId,
  TOAST_VISIBLE_MAX,
  type Toast,
} from "./store";

function mkList(n: number): Toast[] {
  return Array.from({ length: n }, (_, i) => ({
    id: `t_${i}`,
    kind: "info" as const,
    title: `t${i}`,
    durationMs: 1000,
  }));
}

describe("addToast", () => {
  it("appends a new toast with the requested kind and default duration", () => {
    const list = mkList(2);
    const next = addToast(list, { kind: "success", title: "ok" });
    expect(next).toHaveLength(3);
    const tail = next[next.length - 1];
    if (!tail) throw new Error("expected tail");
    expect(tail.kind).toBe("success");
    expect(tail.title).toBe("ok");
    expect(tail.durationMs).toBe(DEFAULT_DURATIONS_MS.success);
  });

  it("uses the per-kind default duration when none is provided", () => {
    const list: Toast[] = [];
    expect(addToast(list, { kind: "error", title: "e" })[0]?.durationMs).toBe(
      DEFAULT_DURATIONS_MS.error,
    );
    expect(
      addToast(list, { kind: "warning", title: "w" })[0]?.durationMs,
    ).toBe(DEFAULT_DURATIONS_MS.warning);
    expect(addToast(list, { kind: "info", title: "i" })[0]?.durationMs).toBe(
      DEFAULT_DURATIONS_MS.info,
    );
  });

  it("honors an explicit durationMs override", () => {
    const list: Toast[] = [];
    const first = addToast(list, {
      kind: "info",
      title: "i",
      durationMs: 1234,
    })[0];
    if (!first) throw new Error("expected first toast");
    expect(first.durationMs).toBe(1234);
  });

  it("caps the list at TOAST_VISIBLE_MAX (5) by dropping the oldest", () => {
    const list = mkList(TOAST_VISIBLE_MAX);
    const next = addToast(list, { kind: "error", title: "overflow" });
    expect(next).toHaveLength(TOAST_VISIBLE_MAX);
    expect(next[0]?.id).toBe("t_1");
    expect(next[next.length - 1]?.title).toBe("overflow");
  });

  it("keeps the list intact when below the cap", () => {
    const list = mkList(TOAST_VISIBLE_MAX - 1);
    const next = addToast(list, { kind: "info", title: "one more" });
    expect(next).toHaveLength(TOAST_VISIBLE_MAX);
    expect(next[0]?.id).toBe("t_0");
  });

  it("assigns a unique id even for back-to-back calls", () => {
    const a = nextToastId();
    const b = nextToastId();
    expect(a).not.toBe(b);
  });
});

describe("dismissToast", () => {
  it("removes the toast with the given id", () => {
    const list = mkList(3);
    const next = dismissToast(list, "t_1");
    expect(next.map((t) => t.id)).toEqual(["t_0", "t_2"]);
  });

  it("is a no-op when the id is not present", () => {
    const list = mkList(2);
    const next = dismissToast(list, "nope");
    expect(next).toHaveLength(2);
  });
});

describe("extractError", () => {
  it("returns an empty message for null / undefined", () => {
    expect(extractError(null).message).toBe("");
    expect(extractError(undefined).message).toBe("");
  });

  it("treats a bare string as the message", () => {
    expect(extractError("boom")).toEqual({ message: "boom" });
  });

  it("reads message from a plain Error", () => {
    expect(extractError(new Error("nope"))).toEqual({ message: "nope" });
  });

  it("extracts code from an ApiClientError-like Error with body", () => {
    const err = Object.assign(new Error("auth_invalid_token"), {
      body: { code: "auth_invalid_token", message: "expired" },
    });
    expect(extractError(err)).toEqual({
      code: "auth_invalid_token",
      message: "expired",
    });
  });

  it("extracts code from a thrown { body: { code, message } } object", () => {
    const err = { body: { code: "boom", message: "failed" } };
    expect(extractError(err)).toEqual({ code: "boom", message: "failed" });
  });

  it("extracts code from a thrown { code, message } object", () => {
    const err = { code: "x", message: "y" };
    expect(extractError(err)).toEqual({ code: "x", message: "y" });
  });

  it("falls back to the Error message when body has no message", () => {
    const err = Object.assign(new Error("plain msg"), {
      body: { code: "x" },
    });
    expect(extractError(err)).toEqual({ code: "x", message: "plain msg" });
  });

  it("ignores empty body / empty object gracefully", () => {
    expect(extractError({})).toEqual({ message: "" });
    expect(extractError({ body: {} })).toEqual({ message: "" });
  });

  it("handles non-string message in body by falling back to top-level", () => {
    const err = { code: "x", message: "top", body: { code: "y" } };
    expect(extractError(err)).toEqual({ code: "y", message: "top" });
  });

  it("stringifies non-error values that are not objects", () => {
    expect(extractError(42)).toEqual({ message: "42" });
  });
});
