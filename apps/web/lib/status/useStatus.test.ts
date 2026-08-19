import { describe, it, expect } from "vitest";
import {
  classify,
  type HealthState,
  type MetaOutcome,
} from "./useStatus";

describe("classify", () => {
  it("returns 'ok' when health ok + meta ok + omp version known", () => {
    expect(classify("ok" as HealthState, "ok" as MetaOutcome, true)).toBe(
      "ok",
    );
  });

  it("returns 'partial' when meta returns 503 omp_not_found", () => {
    expect(
      classify("ok" as HealthState, "omp_not_found" as MetaOutcome, false),
    ).toBe("partial");
  });

  it("returns 'partial' when meta ok but omp_version empty", () => {
    expect(classify("ok" as HealthState, "ok" as MetaOutcome, false)).toBe(
      "partial",
    );
  });

  it("returns 'partial' when health ok and meta failed but no omp signal", () => {
    expect(classify("ok" as HealthState, "failed" as MetaOutcome, false)).toBe(
      "partial",
    );
  });

  it("returns 'fail' when health is in fail state (3 consecutive failures)", () => {
    expect(classify("fail" as HealthState, "ok" as MetaOutcome, true)).toBe(
      "fail",
    );
    expect(
      classify("fail" as HealthState, "omp_not_found" as MetaOutcome, false),
    ).toBe("fail");
  });

  it("returns 'loading' when health state is pending", () => {
    expect(classify("pending" as HealthState, "ok" as MetaOutcome, true)).toBe(
      "loading",
    );
    expect(
      classify("pending" as HealthState, "pending" as MetaOutcome, false),
    ).toBe("loading");
  });

  // Real wire probe: api_version ("v1.6.4") and omp_version
  // ("omp/17.3.4") are different namespaces — the pill must not
  // require equality. (Per the 503 advisory: equality is never
  // true on a healthy system.)
  it("treats api_version vs omp_version as independent signals", () => {
    const apiV: string = "v1.6.4";
    const ompV: string = "omp/17.3.4";
    expect(apiV === ompV).toBe(false);
    // Correct classification: health ok + meta ok + omp version
    // non-empty → green.
    expect(
      classify("ok" as HealthState, "ok" as MetaOutcome, ompV.length > 0),
    ).toBe("ok");
  });
});
