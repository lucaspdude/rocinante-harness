#!/usr/bin/env bash
# Smoke test for PR #6..#11 (search, bulk projects, markdown hardening,
# health indicator, settings anchors, currency conversion).
# Runs against a local roc-harness (api on 30179, web on 30178).
#
# Exit code 0 = OK. Anything else = FAIL.
#
# Override defaults by exporting BASE_API / BASE_WEB / ROCINANTE_PASSPHRASE
# before invoking.

set -euo pipefail

cd "$(dirname "$0")/.."

API="${BASE_API:-http://127.0.0.1:30179}"
WEB="${BASE_WEB:-http://127.0.0.1:30178}"
PASS="${ROCINANTE_PASSPHRASE:-secret-passphrase-1234}"

echo "PR #6..#11: login"
TOKEN=$(curl -sS -X POST "$API/api/v1/login" \
  -H 'Content-Type: application/json' \
  -d "{\"passphrase\":\"$PASS\",\"device_name\":\"smoke-phase-22\"}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["access"])')
[ -n "$TOKEN" ] || { echo "FAIL: could not acquire token"; exit 1; }

echo "PR #6: ripgrep-backed search returns matches"
ROOT=$(curl -sS -H "authorization: Bearer $TOKEN" "$API/api/v1/projects" \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["projects"][0]["path"] if d["projects"] else "")')
[ -n "$ROOT" ] || { echo "FAIL: no projects registered"; exit 1; }
RES=$(curl -sS -X POST "$API/api/v1/search" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"root\":\"$ROOT\",\"pattern\":\"phase\",\"options\":{\"maxResults\":10}}")
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
  -d "{\"op\":\"archive\",\"paths\":[\"$ROOT\"]}")
echo "$RES" | grep -q '"archived":1' || { echo "FAIL: archive expected archived:1, got: $RES"; exit 1; }
# re-register so the smoke is idempotent
curl -fsS -X POST "$API/api/v1/projects" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"path\":\"$ROOT\",\"name\":\"phase-22\"}" >/dev/null || true
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/projects/bulk" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"op\":\"delete\",\"paths\":[\"$ROOT\"]}")
[ "$CODE" = "400" ] || { echo "FAIL: delete without confirmPath expected 400, got $CODE"; exit 1; }
# re-register again so subsequent smokes still have a project
curl -fsS -X POST "$API/api/v1/projects" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"path\":\"$ROOT\",\"name\":\"phase-22\"}" >/dev/null || true

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
