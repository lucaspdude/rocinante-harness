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

# Create two sessions under a real working directory.
WORK="$(mktemp -d)"
S1=$(curl -sX POST http://127.0.0.1:30179/api/v1/sessions \
  -H 'content-type: application/json' \
  -d "{\"omp_cwd\":\"$WORK\"}" | python3 -c 'import json,sys;print(json.load(sys.stdin)["id"])')
S2=$(curl -sX POST http://127.0.0.1:30179/api/v1/sessions \
  -H 'content-type: application/json' \
  -d "{\"omp_cwd\":\"$WORK\"}" | python3 -c 'import json,sys;print(json.load(sys.stdin)["id"])')

# List.
LIST=$(curl -sf http://127.0.0.1:30179/api/v1/sessions)
echo "$LIST" | grep -q "$S1"
echo "$LIST" | grep -q "$S2"

# Title one.
curl -sfX POST http://127.0.0.1:30179/api/v1/sessions/$S1/title \
  -H 'content-type: application/json' \
  -d '{"title":"my first run"}' | grep -q 'my first run'

# Delete.
HTTP=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE http://127.0.0.1:30179/api/v1/sessions/$S2)
test "$HTTP" -eq 204

# After delete, list returns only S1.
LIST=$(curl -sf http://127.0.0.1:30179/api/v1/sessions)
echo "$LIST" | grep -q "$S1"

rm -rf "$WORK"
echo "phase-08 smoke OK"
