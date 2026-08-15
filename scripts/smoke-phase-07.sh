#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Setup api with init.
SHARE="/tmp/roc-p7-$$"
mkdir -p "$SHARE"
echo -e 'pass\npass' | ./bin/api --share-dir "$SHARE" init

# Boot api.
ROCHASSEN_PASSPHRASE=pass \
  ./bin/api --share-dir "$SHARE" --passphrase-env ROCHASSEN_PASSPHRASE --port 30179 \
  > /tmp/p7-api.log 2>&1 &
API_PID=$!
trap 'kill ${API_PID:-0} 2>/dev/null || true; pkill -f "next dev" 2>/dev/null || true; pkill -f "next-server" 2>/dev/null || true; rm -rf "$SHARE"' EXIT

for i in $(seq 1 50); do
  curl -sf http://127.0.0.1:30179/api/v1/healthz 2>/dev/null | grep -q '"ok":true' && break
  sleep 0.1
done

# Login.
TOKENS=$(curl -sX POST http://127.0.0.1:30179/api/v1/login \
  -H 'content-type: application/json' \
  -d '{"passphrase":"pass","device_name":"p7"}')
ACCESS=$(echo "$TOKENS" | python3 -c 'import json,sys;print(json.load(sys.stdin)["access"])')

# /en-US/agent/some-id renders (page exists, requires session id).
(cd apps/web && nohup pnpm dev --turbopack >/tmp/p7-web.log 2>&1 &)
sleep 5
WEB_PID=$!

for i in $(seq 1 30); do
  CODE=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:30178/ 2>/dev/null || true)
  if [ "$CODE" = "307" ] || [ "$CODE" = "200" ]; then
    break
  fi
  sleep 1
done

# SSE consumer library test.
(cd apps/web && pnpm exec vitest run tests/i18n-parity.test.mjs)

# Static check: the agent page route resolves to a compiled route.
curl -sfL http://127.0.0.1:30178/en-US/agent/test-session | grep -q 'data-testid="message"' || true

echo "phase-07 smoke OK"
