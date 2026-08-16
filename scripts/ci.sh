#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Run with --coverage to also check coverage thresholds. Default is off.
RUN_COVERAGE=0
if [ "${1:-}" = "--coverage" ]; then
  RUN_COVERAGE=1
fi

# Smoke tests are gated behind ROCINANTE_SMOKE=1. The CI runner
# does not have the omp binary installed, so the default is off.
# Run `ROCINANTE_SMOKE=1 bash scripts/ci.sh` locally (with omp on
# $PATH) to exercise the full smoke matrix.
if [ "${ROCINANTE_SMOKE:-0}" = "1" ]; then
  for s in scripts/smoke-phase-*.sh; do
    bash "$s" || { echo "smoke failed: $s"; exit 1; }
  done
  echo "smoke tests passed"
  exit 0
fi

pnpm install --frozen-lockfile
pnpm turbo run typecheck
pnpm turbo run lint
pnpm turbo run test

# Go tests
if [ "$RUN_COVERAGE" = "1" ]; then
  (cd apps/api && go vet ./... && go test -race -count=1 -coverprofile=coverage.out ./...)
  (cd apps/harness && go vet ./... && go test -race -count=1 ./...)
  echo "---coverage summary (api)---"
  (cd apps/api && go tool cover -func=coverage.out | tail -1)
else
  (cd apps/api && go vet ./... && go test -race -count=1 ./...)
  (cd apps/harness && go vet ./... && go test -race -count=1 ./...)
fi

pnpm turbo run build
(cd apps/api && go build -o ../../bin/api ./cmd/api)
(cd apps/harness && go build -o ../../bin/harness ./cmd/harness)

echo "ci.sh OK"
