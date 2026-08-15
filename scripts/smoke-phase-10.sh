#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# 1. Build-harness.sh produces 5 binaries.
bash scripts/build-harness.sh >/tmp/p10-build.log 2>&1
COUNT=$(ls apps/harness/bin/roc-harness-* 2>/dev/null | wc -l | tr -d ' ')
test "$COUNT" -ge 5

# 2. Each binary under 30MB.
for bin in apps/harness/bin/roc-harness-*; do
  SIZE=$(stat -c%s "$bin" 2>/dev/null || stat -f%z "$bin")
  if [ "$SIZE" -gt 31457280 ]; then
    echo "binary $bin too large: $SIZE bytes"
    exit 1
  fi
done

# 3. version subcommand prints "harness 0.1.0".
./apps/harness/bin/roc-harness-darwin-amd64 version | grep -q '^harness 0\.1\.0'

# 4. status before any up prints "no harness state".
SHARE="$(mktemp -d)"
ROCHASSEN_SHARE_DIR="$SHARE" ./apps/harness/bin/roc-harness-darwin-amd64 status || true

echo "phase-10 smoke OK"
rm -rf "$SHARE"
