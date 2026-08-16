#!/usr/bin/env bash
# Drive the Playwright E2E suite against an already-running
# roc-harness (api on 30179, web on 30178).
#
# Pre-conditions:
#   - roc-harness is up and listening on 30179 (api) and 30178 (web).
#   - playwright is installed under apps/web (pnpm install).
#
# This script does NOT start the harness — it's meant to be invoked
# from CI after `roc-harness up` is in a ready state, or from a
# local shell with both servers already running.
set -euo pipefail

cd "$(dirname "$0")/.."

REPO_ROOT="$(pwd)"
WEB_DIR="$REPO_ROOT/apps/web"
E2E_DIR="$WEB_DIR/e2e"

if ! curl -sf http://127.0.0.1:30179/api/v1/healthz > /dev/null; then
  echo "api not running on :30179 — start roc-harness first"
  exit 1
fi
if ! curl -sfI http://127.0.0.1:30178/ > /dev/null; then
  echo "web not running on :30178 — start roc-harness first"
  exit 1
fi

cd "$WEB_DIR"
pnpm exec playwright install --with-deps chromium > /dev/null

pnpm exec playwright test \
  --config "$E2E_DIR/playwright.config.ts" \
  "$@"

echo "smoke-e2e OK"
