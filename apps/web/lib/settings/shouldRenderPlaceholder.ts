// Phase 7 — item 03: pure decision logic for the SettingsModal
// "unauthed" placeholder. Extracted from the component so the
// 4 truth-table rows can be unit-tested without React rendering.
//
// The component still controls the actual render; this helper
// is the seam the test exercises.

import type { AuthStatus } from "../auth/auth-status";

export interface PlaceholderInputs {
  // True when localStorage has any token at all.
  token: boolean;
  // True while the useAuthStatus hook's initial fetch is in
  // flight.
  loading: boolean;
  // The resolved auth status (null while loading or on error).
  status: AuthStatus | null;
}

// Returns true when the "Sign in required" placeholder should
// render instead of the modal's section content.
//
// Truth table:
//
//   { token: true, ... }                                       → false
//   { token: false, loading: true }                            → true
//   { token: false, loading: false, status.auth_required: true } → true
//   { token: false, loading: false, status.auth_required: false } → false
//
// Rationale: while the auth status is loading we show the
// placeholder to avoid the auth_missing red-box flash (per
// item 03 AC5). Once the status resolves, we trust it: when
// auth_required is false (api not initialized, onboarding
// mode), the placeholder is wrong; when auth_required is
// true and we have no token, the placeholder is correct.
export function shouldRenderPlaceholder(
  inputs: PlaceholderInputs,
): boolean {
  if (inputs.token) return false;
  if (inputs.loading) return true;
  if (inputs.status?.auth_required) return true;
  return false;
}
