#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# 1. Directory structure.
test -f package.json
test -f pnpm-workspace.yaml
test -f turbo.json
test -d apps/api
test -d apps/web
test -d apps/harness
test -d packages/contract

# 2. Build Go bins.
mkdir -p bin
(cd apps/api && go build -o ../../bin/api ./cmd/api)
(cd apps/harness && go build -o ../../bin/harness ./cmd/harness)

# 3. Versions.
./bin/api --version | grep -q '^api 0\.1\.0'
./bin/harness --version | grep -q '^harness 0\.1\.0'

# 4. /healthz live (api up + curl + kill).
./bin/api --port 30179 --no-encryption &
API_PID=$!
trap 'kill ${API_PID:-0} 2>/dev/null || true' EXIT
for i in $(seq 1 50); do
  curl -sf http://127.0.0.1:30179/api/v1/healthz 2>/dev/null | grep -q '"ok":true' && break
  sleep 0.1
done
curl -sf http://127.0.0.1:30179/api/v1/healthz | grep -q '"ok":true'

# 5. Front dev server (background; bounded wait).
(cd apps/web && nohup pnpm dev --turbopack >/tmp/web-dev.log 2>&1 &)
WEB_PID=$!
trap 'kill ${API_PID:-0} 2>/dev/null || true; pkill -P ${WEB_PID:-0} 2>/dev/null || true; kill ${WEB_PID:-0} 2>/dev/null || true; pkill -f "next dev" 2>/dev/null || true; pkill -f "next-server" 2>/dev/null || true' EXIT
READY=0
for i in $(seq 1 90); do
  if curl -sf http://127.0.0.1:30178/ 2>/dev/null | grep -q 'rocinante-harness — booting'; then
    READY=1
    break
  fi
  sleep 1
done
if [ "$READY" -ne 1 ]; then
  echo "front-web never came up; tail of /tmp/web-dev.log:"
  tail -40 /tmp/web-dev.log
  exit 1
fi

# 6. Grep zero for old fork vocabulary.
HITS=$(git grep -nIE 'ompweb|omp-web|can1357|kahme247|agegr|pi-web|OMP_WEB_' \
  -- ':!**/node_modules' ':!.git' ':!.turbo' ':!.next' ':!LICENSE' \
  ':!docs/**/*.md' ':!**/dist' ':!**/bin' || true)
if [ -n "${HITS}" ]; then
  echo "Found forbidden vocabulary:"
  echo "${HITS}"
  exit 1
fi

echo "phase-00 smoke OK"
