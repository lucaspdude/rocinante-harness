#!/usr/bin/env bash
# Smoke test for PR #4 + #5 (PATCH /files/content + codemirror editor).
# Runs against a local roc-harness (api on 30179, web on 30178).
#
# Exit code 0 = OK. Anything else = FAIL.
#
# Override defaults by exporting BASE_API / BASE_WEB / ROCINANTE_PASSPHRASE
# before invoking.
#
# Note: the PATCH endpoint refuses to create files (no ?create=true). The
# smoke creates the target file on the host via $CREATE_CMD (default: touch
# on the harness ssh alias) before the PATCH.

set -euo pipefail

cd "$(dirname "$0")/.."

API="${BASE_API:-http://127.0.0.1:30179}"
WEB="${BASE_WEB:-http://127.0.0.1:30178}"
PASS="${ROCINANTE_PASSPHRASE:-secret-passphrase-1234}"
CREATE_CMD="${CREATE_CMD:-ssh harness 'touch %s/phase21-target.txt'}"

echo "PR #4: login for token"
TOKEN=$(curl -sS -X POST "$API/api/v1/login" \
  -H 'Content-Type: application/json' \
  -d "{\"passphrase\":\"$PASS\",\"device_name\":\"smoke-phase-21\"}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["access"])')
[ -n "$TOKEN" ] || { echo "FAIL: could not acquire token"; exit 1; }

echo "PR #4: pick a registered project root"
ROOT=$(curl -sS -H "authorization: Bearer $TOKEN" "$API/api/v1/projects" \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["projects"][0]["path"] if d["projects"] else "")')
[ -n "$ROOT" ] || { echo "FAIL: no projects registered"; exit 1; }
ENCODED_ROOT=$(printf %s "$ROOT" | python3 -c 'import sys,urllib.parse;print(urllib.parse.quote(sys.stdin.read()))')

# Best-effort: create the file on the host. Skip silently if the command
# fails (no ssh harness) — the PATCH will then surface file_not_found
# rather than crashing the smoke runner.
printf -v CMD "$CREATE_CMD" "$ROOT"
$CMD >/dev/null 2>&1 || true

echo "PR #4: PATCH happy path -> 200"
curl -fsS -X PATCH "$API/api/v1/files/content?root=$ENCODED_ROOT&path=phase21-target.txt" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"phase-21 hello\n"}' >/dev/null

echo "PR #4: PATCH empty root -> 400 invalid_path"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X PATCH "$API/api/v1/files/content?path=phase21-target.txt" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"x"}')
[ "$CODE" = "400" ] || { echo "FAIL: empty root expected 400, got $CODE"; exit 1; }

echo "PR #4: PATCH .. in path -> 400"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X PATCH "$API/api/v1/files/content?root=$ENCODED_ROOT&path=../escape.txt" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"x"}')
[ "$CODE" = "400" ] || { echo "FAIL: .. path expected 400, got $CODE"; exit 1; }

echo "PR #5: web /en-US/agent renders 200"
CODE=$(curl -sS -o /dev/null -w '%{http_code}' "$WEB/en-US/agent")
[ "$CODE" = "200" ] || { echo "FAIL: /en-US/agent returned $CODE"; exit 1; }

echo "OK phase-21"
