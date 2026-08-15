# Adopters

Conventions this repo chose and why.

## `pnpm` over `npm`/`yarn`

- `pnpm` supports monorepo workspace links via `workspace:*` protocol.
- Faster install and disk-efficient via content-addressable storage.

## `Turborepo` for the pipeline

- Caches task outputs across runs (`turbo.json` `outputs` key).
- Topological task graph avoids running `build` before `typecheck`.

## `go-chi/chi` for the HTTP router

- Stdlib-compatible (no new middleware contract).
- Idiomatic `http.Handler` composition.

## `text/event-stream` (SSE), not WebSocket

- One-directional api→front communication is sufficient.
- Browser-native `EventSource` handles reconnection via `lastEventId`.
- Compatible with HTTP/2 and corporate proxies.
