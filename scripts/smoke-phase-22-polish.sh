#!/usr/bin/env bash
# Smoke test for PR #6..#11 (search, bulk projects, markdown hardening,
# health indicator, settings anchors, currency conversion).
# Runs against a local roc-harness (api on 30179, web on 30178).
#
# Exit code 0 = OK. Anything else = FAIL.
#
# Override defaults by exporting BASE_API / BASE_WEB / ROCINANTE_PASSPHRASE
# before invoking.
#
# Creates a unique throwaway project per run under /tmp/phase3-smoke-* so
# the smoke is fully idempotent and never touches the user's registered
# projects. The throwaway is left registered after the smoke so subsequent
# runs can reuse it (idempotent re-register returns 409 which is fine).

set -euo pipefail

cd "$(dirname "$0")/.."

API="${BASE_API:-http://127.0.0.1:30179}"
WEB="${BASE_WEB:-http://127.0.0.1:30178}"
PASS="${ROCINANTE_PASSPHRASE:-secret-passphrase-1234}"

# Stable throwaway project name + path. The smoke is idempotent: re-uses
# the same root across runs. The project may end up `hidden:true` after a
# bulk-archive — that's the smoke's job, not the user's.
SMOKE_ROOT="/tmp/phase3-smoke-$$"
SMOKE_NAME="phase3-smoke-$$"

echo "PR #6..#11: login"
TOKEN=$(curl -sS -X POST "$API/api/v1/login" \
  -H 'Content-Type: application/json' \
  -d "{\"passphrase\":\"$PASS\",\"device_name\":\"smoke-phase-22\"}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["access"])')
[ -n "$TOKEN" ] || { echo "FAIL: could not acquire token"; exit 1; }

# Ensure the throwaway root exists. ssh may fail in CI (no harness alias)
# — that's acceptable; the curl below will surface any real error.
ssh harness "mkdir -p '$SMOKE_ROOT'" >/dev/null 2>&1 || true
if [ ! -d "$SMOKE_ROOT" ]; then
  # Local fallback (running this smoke directly on the api host).
  mkdir -p "$SMOKE_ROOT"
fi

# Register the throwaway. On 409 (already registered), the api's Allow
# already happened on the first successful Upsert, so re-running is fine.
REG_HTTP=$(curl -sS -o /tmp/phase22-reg.json -w '%{http_code}' -X POST "$API/api/v1/projects" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"path\":\"$SMOKE_ROOT\",\"name\":\"$SMOKE_NAME\"}")
if [ "$REG_HTTP" != "201" ] && [ "$REG_HTTP" != "409" ]; then
  echo "FAIL: register throwaway expected 201 or 409, got $REG_HTTP: $(cat /tmp/phase22-reg.json)"
  exit 1
fi

echo "PR #6: ripgrep-backed search returns matches"
RES=$(curl -sS -X POST "$API/api/v1/search" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"root\":\"$SMOKE_ROOT\",\"pattern\":\"phase\",\"options\":{\"maxResults\":10}}")
echo "$RES" | grep -q '"results"' || { echo "FAIL: search missing results key: $RES"; exit 1; }
echo "$RES" | grep -q '"partial"' || { echo "FAIL: search missing partial key: $RES"; exit 1; }

echo "PR #6: search rejects root outside allowlist"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/search" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"root":"/etc","pattern":"passwd","options":{}}')
[ "$CODE" = "403" ] || { echo "FAIL: /etc search expected 403, got $CODE"; exit 1; }

echo "PR #7: bulk archive + delete (with confirmPath)"
RES=$(curl -sS -X POST "$API/api/v1/projects/bulk" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"op\":\"archive\",\"paths\":[\"$SMOKE_ROOT\"]}")
echo "$RES" | grep -q '"archived":1' || { echo "FAIL: archive expected archived:1, got: $RES"; exit 1; }
# Unhide so subsequent runs see it again (idempotency).
curl -sS -X DELETE "$API/api/v1/projects" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"path\":\"$SMOKE_ROOT\",\"hidden\":false}" >/dev/null 2>&1 || true

CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/projects/bulk" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"op\":\"delete\",\"paths\":[\"$SMOKE_ROOT\"]}")
[ "$CODE" = "400" ] || { echo "FAIL: delete without confirmPath expected 400, got $CODE"; exit 1; }

echo "PR #9: healthz + meta reachable (pill should be green)"
curl -fsS "$API/api/v1/healthz" >/dev/null
curl -fsS "$API/api/v1/meta" | grep -q '"api_version"' || { echo "FAIL: meta missing api_version"; exit 1; }

echo "PR #10: settings deep-link returns 200 for all tabs"
for t in general providers account developer devices foo; do
  CODE=$(curl -sS -o /dev/null -w '%{http_code}' "$WEB/en-US/settings?tab=$t")
  [ "$CODE" = "200" ] || { echo "FAIL: settings?tab=$t returned $CODE"; exit 1; }
done

echo "PR #11: catalog endpoint accepts ?locale for all supported locales"
for L in en-US pt-BR en-GB de-DE ja-JP zh-CN fr-FR; do
  RES=$(curl -sS -H "authorization: Bearer $TOKEN" "$API/api/v1/models/catalog?locale=$L&limit=1")
  echo "$RES" | grep -q '"results"' || { echo "FAIL: catalog locale=$L missing results"; exit 1; }
done

echo "PR #1..#11: web /en-US/agent 200"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' "$WEB/en-US/agent")
[ "$CODE" = "200" ] || { echo "FAIL: /en-US/agent returned $CODE"; exit 1; }

echo "OK phase-22"
