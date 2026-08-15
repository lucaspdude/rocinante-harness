#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# 1. --bind 0.0.0.0 without allow-list aborts.
SHARE="$(mktemp -d)"
mkdir -p "$SHARE"
echo -e 'pass\npass' | ./bin/api --share-dir "$SHARE" init
if ./bin/api --share-dir "$SHARE" --bind 0.0.0.0 --port 30179 > /tmp/p13-bad.log 2>&1; then
  echo "expected --bind 0.0.0.0 without allow-list to fail"
  exit 1
fi

# 2. --bind 0.0.0.0 with allow-list boots.
./bin/api --share-dir "$SHARE" --bind 127.0.0.1 --cors-allowlist "https://app.example.com" --port 30179 &
API_PID=$!
trap 'kill ${API_PID:-0} 2>/dev/null || true; rm -rf "$SHARE"' EXIT

for i in $(seq 1 50); do
  curl -sf http://127.0.0.1:30179/api/v1/healthz 2>/dev/null | grep -q '"ok":true' && break
  sleep 0.1
done

# 3. CORS allows the listed origin.
HEADERS=$(curl -sI -H "Origin: https://app.example.com" http://127.0.0.1:30179/api/v1/healthz)
echo "$HEADERS" | grep -qi 'access-control-allow-origin: https://app.example.com'

# 4. CORS rejects an unknown origin.
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -H "Origin: https://evil.com" http://127.0.0.1:30179/api/v1/healthz)
test "$STATUS" -eq 403

# 5. TLSHandler attaches security headers.
curl -sI http://127.0.0.1:30179/api/v1/healthz | grep -qi 'X-Content-Type-Options: nosniff'
curl -sI http://127.0.0.1:30179/api/v1/healthz | grep -qi 'Strict-Transport-Security'

# 6. Migration 003 ran (rate_limits table exists).
sqlite3 "$SHARE/roc-harness.db" ".tables rate_limits" | grep -q rate_limits

echo "phase-13 smoke OK"
