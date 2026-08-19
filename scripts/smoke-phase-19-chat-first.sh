#!/usr/bin/env bash
# Smoke test for PR #1 + #2 (toast infra + chat-first layout).
# Runs against a local roc-harness (api on 30179, web on 30178).
#
# Exit code 0 = OK. Anything else = FAIL.
#
# Override defaults by exporting BASE_API / BASE_WEB before invoking.

set -euo pipefail

cd "$(dirname "$0")/.."

API="${BASE_API:-http://127.0.0.1:30179}"
WEB="${BASE_WEB:-http://127.0.0.1:30178}"

echo "PR #1: toast infra"
RES=$(curl -fsS -X POST "$API/api/v1/projects" \
  -H 'Content-Type: application/json' \
  -d '{"path":"~","name":"toast-test"}' || true)
echo "$RES" | grep -q 'auth_missing' || { echo "FAIL: not 401"; exit 1; }
# Web side: open /en-US/settings?tab=providers, fire saveKey with empty value,
# expect a toast (manual via playwright)

echo "PR #2: chat-first"
# /agent/new must be a real HTML response (200) — the redirect is
# client-side, but the server should still respond (Next.js renders
# the client redirect component as 200).
CODE=$(curl -fsS -o /dev/null -w '%{http_code}' "$WEB/en-US/agent/new")
[ "$CODE" = "200" ] || { echo "FAIL: /en-US/agent/new returned $CODE"; exit 1; }

# /agent is the new chat-first home — must return 200.
CODE=$(curl -fsS -o /dev/null -w '%{http_code}' "$WEB/en-US/agent")
[ "$CODE" = "200" ] || { echo "FAIL: /en-US/agent returned $CODE"; exit 1; }

echo "OK phase-19"
