#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

pnpm install --frozen-lockfile

pnpm turbo run typecheck
pnpm turbo run lint
pnpm turbo run test

(cd apps/api && go vet ./... && go test -race -count=1 ./...)
(cd apps/harness && go vet ./... && go test -race -count=1 ./...)

pnpm turbo run build
(cd apps/api && go build -o ../../bin/api ./cmd/api)
(cd apps/harness && go build -o ../../bin/harness ./cmd/harness)

for s in scripts/smoke-phase-*.sh; do
  bash "$s" || { echo "smoke failed: $s"; exit 1; }
done

echo "ci.sh OK"
