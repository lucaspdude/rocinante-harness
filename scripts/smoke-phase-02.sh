#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

mkdir -p bin
(cd apps/api && go build -o ../../bin/api ./cmd/api)

./bin/api --no-encryption --port 30179 &
API_PID=$!
trap 'kill ${API_PID:-0} 2>/dev/null || true' EXIT

for i in $(seq 1 50); do
  curl -sf http://127.0.0.1:30179/api/v1/healthz 2>/dev/null | grep -q '"ok":true' && break
  sleep 0.1
done

# Create a session.
SESSION=$(curl -sX POST http://127.0.0.1:30179/api/v1/sessions \
  -H 'content-type: application/json' \
  -d '{"omp_cwd":"/tmp"}' | python3 -c 'import json,sys;print(json.load(sys.stdin)["id"])')

# Consume the SSE stream for 2 seconds.
# Consume the SSE stream for 2 seconds via curl --max-time.
curl -Ns --max-time 2 http://127.0.0.1:30179/api/v1/sessions/$SESSION/events \
  > /tmp/sse.out 2>&1 || true
test -s /tmp/sse.out
grep -q '^data: ' /tmp/sse.out

# 404 on missing session.
HTTP_404=$(curl -s -o /dev/null -w '%{http_code}' \
  http://127.0.0.1:30179/api/v1/sessions/inexistente/events)
test "$HTTP_404" -eq 404

echo "phase-02 smoke OK"
