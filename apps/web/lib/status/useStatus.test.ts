import { describe, it, expect } from "vitest";
import { classify } from "./useStatus";

describe("classify", () => {
  it("returns 'ok' when both versions match", () => {
    expect(classify("v1.6.4", "v1.6.4")).toBe("ok");
  });

  it("returns 'partial' when both versions are known but differ", () => {
    expect(classify("v1.6.4", "v1.6.5")).toBe("partial");
  });

  it("returns 'partial' when only api version is known", () => {
    expect(classify("v1.6.4", "")).toBe("partial");
  });

  it("returns 'loading' when nothing is known yet", () => {
    expect(classify("", "")).toBe("loading");
  });
});
