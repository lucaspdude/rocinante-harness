#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Setup share-dir with init.
SHARE="$(mktemp -d)"
mkdir -p "$SHARE"
echo -e 'pass\npass' | ./bin/api --share-dir "$SHARE" init

# Boot api with passphrase.
ROCHASSEN_PASSPHRASE=pass \
  ./bin/api --share-dir "$SHARE" --passphrase-env ROCHASSEN_PASSPHRASE --port 30179 \
  > /tmp/p12-api.log 2>&1 &
API_PID=$!
trap 'kill ${API_PID:-0} 2>/dev/null || true; rm -rf "$SHARE"' EXIT

for i in $(seq 1 50); do
  curl -sf http://127.0.0.1:30179/api/v1/healthz 2>/dev/null | grep -q '"ok":true' && break
  sleep 0.1
done

# Login.
TOKENS=$(curl -sX POST http://127.0.0.1:30179/api/v1/login \
  -H 'content-type: application/json' \
  -d '{"passphrase":"pass","device_name":"p12"}')
ACCESS=$(echo "$TOKENS" | python3 -c 'import json,sys;print(json.load(sys.stdin)["access"])')

# /api/v1/ssh/keys requires auth.
STATUS=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:30179/api/v1/ssh/keys)
test "$STATUS" -eq 401

# Create a key.
KEY=$(curl -sfX POST http://127.0.0.1:30179/api/v1/ssh/keys \
  -H "Authorization: Bearer $ACCESS" \
  -H 'content-type: application/json' \
  -d '{"label":"github","provider":"github"}')
KEY_ID=$(echo "$KEY" | python3 -c 'import json,sys;print(json.load(sys.stdin)["id"])')
echo "$KEY" | grep -q '"private_key"'
echo "$KEY" | grep -q 'OPENSSH PRIVATE KEY'

# List keys.
KEYS=$(curl -sf -H "Authorization: Bearer $ACCESS" http://127.0.0.1:30179/api/v1/ssh/keys)
echo "$KEYS" | grep -q "$KEY_ID"

# Create a server.
SERVER=$(curl -sfX POST http://127.0.0.1:30179/api/v1/ssh/servers \
  -H "Authorization: Bearer $ACCESS" \
  -H 'content-type: application/json' \
  -d "{\"label\":\"localhost\",\"host\":\"127.0.0.1\",\"port\":22,\"username\":\"lucas\",\"key_id\":\"$KEY_ID\"}")
SERVER_ID=$(echo "$SERVER" | python3 -c 'import json,sys;print(json.load(sys.stdin)["id"])')

# List servers.
SERVERS=$(curl -sf -H "Authorization: Bearer $ACCESS" http://127.0.0.1:30179/api/v1/ssh/servers)
echo "$SERVERS" | grep -q "$SERVER_ID"

# Delete key + server.
curl -sfX DELETE -H "Authorization: Bearer $ACCESS" "http://127.0.0.1:30179/api/v1/ssh/keys/$KEY_ID"
curl -sfX DELETE -H "Authorization: Bearer $ACCESS" "http://127.0.0.1:30179/api/v1/ssh/servers/$SERVER_ID"

echo "phase-12 smoke OK"
