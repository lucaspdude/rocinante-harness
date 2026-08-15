#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# 1. Installer script is syntactically valid.
bash -n installer/install.sh

# 2. ROCHASSEN_SKIP_INIT=1 path validates too.
ROCHASSEN_SKIP_INIT=1 bash -n installer/install.sh

# 3. /api/v1/onboarding/status returns the expected shape.
SHARE="$(mktemp -d)"
mkdir -p "$SHARE"
./bin/api --no-encryption --share-dir "$SHARE" init

./bin/api --no-encryption --share-dir "$SHARE" --port 30179 &
API_PID=$!
trap 'kill ${API_PID:-0} 2>/dev/null || true; rm -rf "$SHARE"' EXIT

for i in $(seq 1 50); do
  curl -sf http://127.0.0.1:30179/api/v1/healthz 2>/dev/null | grep -q '"ok":true' && break
  sleep 0.1
done

STATUS=$(curl -sf http://127.0.0.1:30179/api/v1/onboarding/status)
echo "$STATUS" | grep -q '"initialized":true'
echo "$STATUS" | grep -q '"requires_setup":false'
echo "$STATUS" | grep -q '"api_version":"0.1.0"'

# 4. Status before init returns requires_setup=true.
SHARE2="$(mktemp -d)"
mkdir -p "$SHARE2"
./bin/api --no-encryption --share-dir "$SHARE2" --port 30180 &
API2_PID=$!
trap 'kill ${API_PID:-0} ${API2_PID:-0} 2>/dev/null || true; rm -rf "$SHARE" "$SHARE2"' EXIT

for i in $(seq 1 50); do
  curl -sf http://127.0.0.1:30180/api/v1/healthz 2>/dev/null | grep -q '"ok":true' && break
  sleep 0.1
done

STATUS=$(curl -sf http://127.0.0.1:30180/api/v1/onboarding/status)
echo "$STATUS" | grep -q '"initialized":false'
echo "$STATUS" | grep -q '"requires_setup":true'

echo "phase-11 smoke OK"
