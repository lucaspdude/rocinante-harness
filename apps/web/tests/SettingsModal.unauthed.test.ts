// Phase 7 — item 03: pure unit test for the shouldRenderPlaceholder
// decision logic. The SettingsModal component would otherwise
// require jsdom + @testing-library/react; this 4-case test
// exercises the contract without React rendering.

import { describe, it, expect } from "vitest";
import { shouldRenderPlaceholder } from "../lib/settings/shouldRenderPlaceholder";

describe("shouldRenderPlaceholder (SettingsModal auth gate)", () => {
  it("returns false when the user has a token", () => {
    expect(
      shouldRenderPlaceholder({
        token: true,
        loading: false,
        status: { initialized: true, auth_required: true, device_known: true },
      }),
    ).toBe(false);
  });

  it("returns true while auth status is loading", () => {
    expect(
      shouldRenderPlaceholder({
        token: false,
        loading: true,
        status: null,
      }),
    ).toBe(true);
  });

  it("returns true when no token and auth is required", () => {
    expect(
      shouldRenderPlaceholder({
        token: false,
        loading: false,
        status: { initialized: true, auth_required: true, device_known: false },
      }),
    ).toBe(true);
  });

  it("returns false when no token and auth is NOT required (onboarding)", () => {
    expect(
      shouldRenderPlaceholder({
        token: false,
        loading: false,
        status: { initialized: false, auth_required: false, device_known: false },
      }),
    ).toBe(false);
  });
});
