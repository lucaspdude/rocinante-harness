#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

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
(cd apps/api && go vet ./... && go test -race -count=1 ./...)
(cd apps/harness && go vet ./... && go test -race -count=1 ./...)
pnpm turbo run build
(cd apps/api && go build -o ../../bin/api ./cmd/api)
(cd apps/harness && go build -o ../../bin/harness ./cmd/harness)

echo "ci.sh OK"
