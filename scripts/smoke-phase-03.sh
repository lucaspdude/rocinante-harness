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

# Prompt with idempotency key.
KEY=$(uuidgen)
R1=$(curl -sX POST http://127.0.0.1:30179/api/v1/sessions/$SESSION/prompt \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: $KEY" \
  -d '{"text":"echo hello"}')
R2=$(curl -sX POST http://127.0.0.1:30179/api/v1/sessions/$SESSION/prompt \
  -H 'content-type: application/json' \
  -H "Idempotency-Key: $KEY" \
  -d '{"text":"echo hello"}')
test "$R1" = "$R2"

# Check best-effort header on a fresh request.
# Use -D to dump headers of the POST response.
HEADERS=$(curl -s -D - -X POST http://127.0.0.1:30179/api/v1/sessions/$SESSION/prompt \
  -H "Idempotency-Key: $(uuidgen)" -H 'content-type: application/json' \
  -d '{"text":"x"}')
echo "$HEADERS" | grep -qi 'X-Idempotency-Cache-State: best-effort'

# Abort a session.
ABORT=$(curl -sX POST http://127.0.0.1:30179/api/v1/sessions/$SESSION/abort)
echo "$ABORT" | grep -q '"aborted":true'

# 404 on missing.
HTTP_404=$(curl -s -o /dev/null -w '%{http_code}' \
  -X POST http://127.0.0.1:30179/api/v1/sessions/inexistente/prompt \
  -H 'content-type: application/json' -d '{"text":"x"}')
test "$HTTP_404" -eq 404

echo "phase-03 smoke OK"
