#!/usr/bin/env bash
# Smoke test for PR #3 (picker recovery + public /api/v1/me).
# Runs against a local roc-harness (api on 30179).
#
# Exit code 0 = OK. Anything else = FAIL.
#
# Override defaults by exporting BASE_API / BASE_WEB before invoking.

set -euo pipefail

cd "$(dirname "$0")/.."

API="${BASE_API:-http://127.0.0.1:30179}"
WEB="${BASE_WEB:-http://127.0.0.1:30178}"

echo "PR #3: /me is public (no token)"
RES=$(curl -fsS "$API/api/v1/me")
echo "$RES" | grep -q '"home"' || { echo "FAIL: /me body missing home field: $RES"; exit 1; }
echo "$RES" | grep -q '"user"' || { echo "FAIL: /me body missing user field: $RES"; exit 1; }
echo "$RES" | grep -q '"host"' || { echo "FAIL: /me body missing host field: $RES"; exit 1; }

echo "PR #3: cwd/browse with no token still requires auth"
RES=$(curl -sS -o /dev/null -w '%{http_code}' "$API/api/v1/cwd/browse?path=/")
[ "$RES" = "401" ] || { echo "FAIL: expected 401 for cwd/browse, got $RES"; exit 1; }

echo "PR #3: web /en-US/agent page renders"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' "$WEB/en-US/agent")
[ "$CODE" = "200" ] || { echo "FAIL: /en-US/agent returned $CODE"; exit 1; }

echo "OK phase-20"