#!/usr/bin/env bash
# Phase 8 — regression smoke for the AuthMW wiring bug.
#
# Background: the phase-8 merge accidentally removed the
# `AuthMW: authMW` field from the RouterDeps struct literal in
# apps/api/cmd/api/main.go. With AuthMW zero-valued, NewRouter's
# `if deps.AuthMW != nil` guard skipped the entire auth-protected
# group, so /api/v1/projects, /api/v1/devices, /api/v1/logout
# silently returned 404. /api/v1/meta (which still lives
# outside the auth group) appeared to work, masking the bug.
#
# This smoke catches the regression by POSTing /api/v1/projects
# with NO Bearer token and asserting we get 401 (auth group
# mounted, route present, token missing) and NOT 404 (route
# absent because the auth group was skipped).
#
# Exit code 0 = OK. Anything else = FAIL.
#
# Override defaults by exporting BASE_API / ROCINANTE_PASSPHRASE.

set -euo pipefail

cd "$(dirname "$0")/.."

API="${BASE_API:-http://127.0.0.1:30179}"

echo "Phase 8 — auth-protected group is mounted"

# 1. Unauthenticated POST /api/v1/projects must return 401 (route
#    registered; auth group is mounted). NOT 404 (group skipped).
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/projects" \
  -H 'Content-Type: application/json' \
  -d '{}')
if [ "$CODE" = "404" ]; then
  echo "FAIL: POST /api/v1/projects returned 404 — auth group was not mounted (AuthMW was not wired)"
  exit 1
fi
if [ "$CODE" != "401" ]; then
  echo "FAIL: POST /api/v1/projects returned $CODE, want 401"
  exit 1
fi

# 2. Same for /api/v1/devices (GET inside the auth group).
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X GET "$API/api/v1/devices")
if [ "$CODE" = "404" ]; then
  echo "FAIL: GET /api/v1/devices returned 404 — auth group was not mounted"
  exit 1
fi

# 3. Same for POST /api/v1/logout.
CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/logout")
if [ "$CODE" = "404" ]; then
  echo "FAIL: POST /api/v1/logout returned 404 — auth group was not mounted"
  exit 1
fi

# 4. With a valid Bearer token, the route should succeed (not 401).
#    We only assert it's NOT a 404 — auth flow is exercised in
#    other phases; this smoke is just about the wiring.
PASS="${ROCINANTE_PASSPHRASE:-secret-passphrase-1234}"
TOKEN=$(curl -sS -X POST "$API/api/v1/login" \
  -H 'Content-Type: application/json' \
  -d "{\"passphrase\":\"$PASS\",\"device_name\":\"smoke-phase-24\"}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["access"])' 2>/dev/null || echo "")
if [ -n "$TOKEN" ]; then
  CODE=$(curl -sS -o /dev/null -w '%{http_code}' -X GET "$API/api/v1/devices" \
    -H "authorization: Bearer $TOKEN")
  if [ "$CODE" = "404" ]; then
    echo "FAIL: GET /api/v1/devices (with auth) returned 404"
    exit 1
  fi
fi

echo "OK phase-24"
