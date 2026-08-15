# Architecture

The product `rocinante-harness` is split into independent packs:

- `apps/api` (Go) — single-binary HTTP service that talks to `omp` via stdio
  NDJSON and exposes a REST + SSE surface to clients.
- `apps/web` (Next.js 16) — single-binary web UI; consumes `apps/api` over
  HTTP. SSR handles locale/theme pre-hydration.
- `apps/harness` (Go) — umbrella launcher that orchestrates `api` + `web`
  as sibling processes, manages PID files, logs, and graceful shutdown.
- `packages/contract` (TS) — shared types and endpoint helpers used by
  both front and api (api imports via `internal/contractgen` in Go).
- `installer/` — `curl | sh` shell script that downloads release binaries
  and runs the onboarding wizard.

## Decision frames

- **D-FRAME-1..14**: `docs/experiments/CONTEXT.md`, `docs/experiments/2026-08-15-sub-grills.md`.
- **ADR 0005**: drop ompweb, build fresh.
- **ADR 0006**: passphrase-wrapped Ed25519 + device-bound refresh.
- **ADR 0007**: cross-cutting coding standards (lint, comments, tests).

## Path-prefix invariant

Every public endpoint the front consumes is `/api/v1/...`. The canonical
list lives in `packages/contract/src/endpoints.ts`. Go reuses the same
`API` constant via `internal/contractgen` (P0 placeholder).

## Phase plan

See `docs/experiments/phases/README.md` for the 15-phase delivery plan.
