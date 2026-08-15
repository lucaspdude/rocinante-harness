#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Build the api binary (idempotent).
mkdir -p bin
(cd apps/api && go build -o ../../bin/api ./cmd/api)

# Boot api.
./bin/api --no-encryption --port 30179 &
API_PID=$!
trap 'kill ${API_PID:-0} 2>/dev/null || true' EXIT

# Wait for /api/v1/healthz to come up.
for i in $(seq 1 50); do
  curl -sf http://127.0.0.1:30179/api/v1/healthz 2>/dev/null | grep -q '"ok":true' && break
  sleep 0.1
done

# /api/v1/meta returns 200 with api_version; omp_version may be empty
# when omp is not installed on the host. Both 200 and 503 are
# accepted; what matters is the body, not the status.
STATUS=$(curl -s -o /tmp/meta-resp.json -w '%{http_code}' http://127.0.0.1:30179/api/v1/meta)
BODY=$(cat /tmp/meta-resp.json)
if [ "${STATUS}" != "200" ] && [ "${STATUS}" != "503" ]; then
  echo "unexpected status ${STATUS} for /api/v1/meta: ${BODY}"
  exit 1
fi
echo "${BODY}" | grep -q '"api_version":"0.1.0"'

echo "phase-01 smoke OK"
