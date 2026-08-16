# Architecture

`rocinante-harness` is a self-hosted AI agent product split into
independent packs.

## Packs

| Pack | Type | Purpose |
|------|------|---------|
| `apps/api` | Go 1.22+ | Single-binary HTTP service. Talks to `omp` via stdio NDJSON and exposes REST + SSE. |
| `apps/web` | Next.js 16 + TS | Single-binary web UI. Consumes `apps/api` over HTTP. SSR handles locale/theme pre-hydration. |
| `apps/harness` | Go 1.22+ | `roc-harness` umbrella launcher that orchestrates `api` + `web` as sibling processes. PID files, logs, graceful shutdown. |
| `packages/contract` | TS | Shared types and endpoint helpers used by both front and api. Go reuses via `internal/contractgen`. |
| `installer/` | Bash | `curl \| sh` bootstrap. Downloads release binaries and runs the onboarding wizard. |

## Runtime topology

```
┌────────────────────────────────────────────────────┐
│                  host machine                      │
│                                                    │
│   ┌──────────────┐  stdio NDJSON   ┌────────────┐  │
│   │ apps/api     │◀───────────────▶│  omp       │  │
│   │ (port 30179) │                 │  (binary)  │  │
│   │              │                 └────────────┘  │
│   │              │                                 │
│   │              │── .ed25519 ──┐                  │
│   │              │── roc-harness.db (sqlite)       │
│   │              │── logs/                        │
│   └──────┬───────┘                                 │
│          │ HTTP                                    │
│          │                                         │
│   ┌──────▼───────┐                                 │
│   │ apps/web     │                                 │
│   │ (port 30178) │                                 │
│   │              │                                 │
│   └──────┬───────┘                                 │
│          │                                         │
└──────────┼─────────────────────────────────────────┘
           │ HTTPS (caddy / cloudflared)
           ▼
       browser
```

`roc-harness` is a thin supervisor that starts both binaries and
forwards signals. It does NOT proxy requests between them — `apps/web`
talks to `apps/api` directly.

## Decision frames

- **D-FRAME-1..14**: `docs/experiments/CONTEXT.md`,
  `docs/experiments/2026-08-15-sub-grills.md`.
- **ADR 0005**: drop ompweb, build fresh.
- **ADR 0006**: passphrase-wrapped Ed25519 + device-bound refresh.
- **ADR 0007**: cross-cutting coding standards (lint, comments, tests).

## Path-prefix invariant

Every public endpoint the front consumes is `/api/v1/...`. The
canonical list lives in `packages/contract/src/endpoints.ts`. Go
reuses the same `API` constant via `internal/contractgen` (P0
placeholder).

## SSE protocol (P2)

The api forwards omp's NDJSON frames **1:1** on the session stream
(D-FRAME-8). No translation, no re-mapping, no envelope. Frames
carry `id:` (omp's `seq`) for reconnect support. See
`docs/experiments/2026-08-15-sub-grills.md` §A.

## Auth lifecycle (P4 + P5)

The api generates an Ed25519 keypair wrapped with a passphrase
derived from the user. The passphrase is the single secret: every
device gets a device-bound refresh token, and every JWT access
token is signed with the Ed25519 private key. Pairing a new device
requires a short-lived code from the owner device.

```
passphrase ── PBKDF2 ── Ed25519 secret ── JWT (access)
                                    └─ refresh tokens (per device)
                                    └─ pairing codes (one-shot, 5min)
```

## Front-end layering

- **`apps/web/app/[locale]/login`** — passphrase + device name.
- **`apps/web/app/[locale]/onboarding`** — first-run setup.
- **`apps/web/app/[locale]/settings`** — General / Account / Devices tabs.
- **`apps/web/app/[locale]/agent/[id]`** — chat surface; SSE stream.

Layout shells (Sidebar, Settings tabs) live under
`apps/web/app/[locale]/agent/` and `apps/web/app/[locale]/settings/`.

## Phase plan

See `docs/experiments/phases/README.md` for the 15-phase delivery
plan and `docs/experiments/2026-08-15-roc-inante-harness.md` for
the broader motivation.
