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
# Use -sS (not -fsS) so the body is captured even when the api returns
# 401 with the auth_missing code — `-f` would abort on 4xx and we'd
# never see the body to grep.
RES=$(curl -sS -X POST "$API/api/v1/projects" \
  -H 'Content-Type: application/json' \
  -d '{"path":"~","name":"toast-test"}')
echo "$RES" | grep -q 'auth_missing' || { echo "FAIL: expected auth_missing, got: $RES"; exit 1; }
# Web side: open /en-US/settings?tab=providers, fire saveKey with empty value,
# expect a toast (manual via playwright)

echo "PR #2: chat-first"
# /agent/new is the 5-line client redirect. The server still renders the
# client component as 200 (the redirect happens in a useEffect).
CODE=$(curl -sS -o /dev/null -w '%{http_code}' "$WEB/en-US/agent/new")
[ "$CODE" = "200" ] || { echo "FAIL: /en-US/agent/new returned $CODE"; exit 1; }

# /agent is the new chat-first home — must return 200.
CODE=$(curl -sS -o /dev/null -w '%{http_code}' "$WEB/en-US/agent")
[ "$CODE" = "200" ] || { echo "FAIL: /en-US/agent returned $CODE"; exit 1; }

echo "OK phase-19"
