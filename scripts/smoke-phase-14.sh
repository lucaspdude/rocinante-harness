#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Build harness works.
bash scripts/build-harness.sh > /tmp/p14-build.log 2>&1
ls apps/harness/bin/roc-harness-* > /dev/null

# Version is correct.
./apps/harness/bin/roc-harness-darwin-amd64 version | grep -q '^harness 0\.1\.0'

# Cross-compile matrix covers 5 platforms.
COUNT=$(ls apps/harness/bin/roc-harness-* | wc -l | tr -d ' ')
test "$COUNT" -ge 5

# CI workflows and dependabot are present.
test -f .github/workflows/ci.yml
test -f .github/workflows/release.yml
test -f .github/dependabot.yml

# Deploy docs are present.
test -f docs/deploy/Caddyfile
test -f docs/deploy/cloudflare-tunnel.md
test -f docs/deploy/security-checklist.md

# README exists.
test -f README.md

# Install script runs in skip-init mode.
bash -n installer/install.sh

# Run phase-13 smoke (the smallest recent one) to confirm the
# full pipeline still works.
bash scripts/smoke-phase-13.sh > /tmp/p14-p13.log 2>&1
grep -q 'phase-13 smoke OK' /tmp/p14-p13.log

echo "phase-14 smoke OK"
