# Design

Concise spec for the user-facing behaviour of `rocinante-harness`.

## Packs

- **api**: listens on `127.0.0.1:30179` by default. Single binary,
  cross-platform (mac, linux, windows). Talks to `omp` via stdio NDJSON.
- **front-web**: listens on `127.0.0.1:30178` by default. Single binary
  Next.js server. Two locales: `en-US`, `pt-BR`.
- **roc-harness**: orchestrates `api` + `web` as sibling processes.
  Single binary, cross-platform.

## Auth

- Passphrase-based login → Ed25519 access token (1h) + opaque refresh
  token (30d, rotation on use).
- Token store is per-device. Refresh reuse revokes the whole family.
- Pairing code (8 alnum chars, 5min, single-use) for new-device setup.

## Sessions

- `POST /api/v1/sessions` spawns `omp` with `omp_cwd`.
- `GET /api/v1/sessions/{id}/events` is the SSE stream — 1:1 with the
  NDJSON frames `omp` produces.
- `POST /api/v1/sessions/{id}/prompt|abort|fork` send NDJSON frames.

## Storage

- `~/.local/share/rocinante-harness/` (Linux/macOS) holds the
  passphrase-wrapped Ed25519 key, SQLite DB, and logs.
- Windows: `%LOCALAPPDATA%\rocinante-harness\`.
- Override with `--share-dir PATH` or env `ROCHASSEN_SHARE_DIR`.

## Out-of-scope (v0.1.0)

- Front-nativo (Tauri/Electron): post-MVP.
- WebSocket: SSE is the only api↔front channel.
- Provider OAuth for SSH publish: post-MVP.
