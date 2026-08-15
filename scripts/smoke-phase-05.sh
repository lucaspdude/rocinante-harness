#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

mkdir -p bin
(cd apps/api && go build -o ../../bin/api ./cmd/api)

# Setup share-dir with an init.
SHARE="/tmp/roc-auth-test-$$"
mkdir -p "$SHARE"
echo -e 'pass\npass' | ./bin/api --share-dir "$SHARE" init
test -f "$SHARE/.ed25519"

# Boot api with passphrase env.
ROCHASSEN_PASSPHRASE=pass \
  ./bin/api --share-dir "$SHARE" --passphrase-env ROCHASSEN_PASSPHRASE --port 30179 \
  > /tmp/api.log 2>&1 &
API_PID=$!
trap 'kill ${API_PID:-0} 2>/dev/null || true; rm -rf "$SHARE"' EXIT

for i in $(seq 1 50); do
  curl -sf http://127.0.0.1:30179/api/v1/healthz 2>/dev/null | grep -q '"ok":true' && break
  sleep 0.1
done

# Login.
TOKENS=$(curl -sX POST http://127.0.0.1:30179/api/v1/login \
  -H 'content-type: application/json' \
  -d '{"passphrase":"pass","device_name":"smoke"}')
ACCESS=$(echo "$TOKENS" | python3 -c 'import json,sys;print(json.load(sys.stdin)["access"])')
REFRESH=$(echo "$TOKENS" | python3 -c 'import json,sys;print(json.load(sys.stdin)["refresh"])')
test -n "$ACCESS"
test -n "$REFRESH"

# Wrong passphrase → 401.
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:30179/api/v1/login \
  -H 'content-type: application/json' \
  -d '{"passphrase":"wrong"}')
test "$STATUS" -eq 401

# /devices requires auth.
STATUS=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:30179/api/v1/devices)
test "$STATUS" -eq 401

# With auth → 200.
DEVICES=$(curl -sf -H "Authorization: Bearer $ACCESS" http://127.0.0.1:30179/api/v1/devices)
echo "$DEVICES" | python3 -c 'import json,sys;assert len(json.load(sys.stdin)["devices"]) >= 1'

# Refresh.
NEW=$(curl -sX POST http://127.0.0.1:30179/api/v1/refresh \
  -H 'content-type: application/json' \
  -d "{\"refresh\":\"$REFRESH\"}")
NEW_ACCESS=$(echo "$NEW" | python3 -c 'import json,sys;print(json.load(sys.stdin)["access"])')
test -n "$NEW_ACCESS"

# Pairing init requires auth.
CODE=$(curl -sf -X POST http://127.0.0.1:30179/api/v1/pairing/init \
  -H "Authorization: Bearer $ACCESS" \
  -H 'content-type: application/json' -d '{}')
PAIRING=$(echo "$CODE" | python3 -c 'import json,sys;print(json.load(sys.stdin)["code"])')
test -n "$PAIRING"
test ${#PAIRING} -eq 8

# Pairing redeem.
REDEEM=$(curl -sX POST http://127.0.0.1:30179/api/v1/pairing/redeem \
  -H 'content-type: application/json' \
  -d "{\"code\":\"$PAIRING\",\"device_name\":\"newlaptop\"}")
echo "$REDEEM" | python3 -c 'import json,sys;assert json.load(sys.stdin)["device_id"]'

# Second redeem → 404.
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:30179/api/v1/pairing/redeem \
  -H 'content-type: application/json' \
  -d "{\"code\":\"$PAIRING\"}")
test "$STATUS" -eq 404

echo "phase-05 smoke OK"
