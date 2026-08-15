# rocinante-harness

A self-hosted AI agent product.

Three packages:

- `apps/api` — Go HTTP service that talks to `omp` via stdio
  NDJSON and exposes a REST + SSE surface to clients.
- `apps/web` — Next.js 16 UI that talks to the api.
- `apps/harness` — `roc-harness` umbrella launcher that brings up
  the api + web as sibling processes.

## 30-second quick start

```bash
# Install the api and harness binaries into ~/.local/share/rocinante-harness/bin.
curl -fsSL https://raw.githubusercontent.com/lucaspdude/rocinante-harness/main/installer/install.sh | bash

# Initialize the passphrased key + SQLite database.
~/.local/share/rocinante-harness/bin/api init

# Bring up api + web. SIGINT stops both.
~/.local/share/rocinante-harness/bin/roc-harness up
```

By default the api listens on `http://127.0.0.1:30179` and the web on
`http://127.0.0.1:30178`. Open the web URL in a browser, enter the
passphrase, and the first session is created.

## Repository layout

```
apps/
  api/                 # Go API
    internal/api/      # chi router + handlers
    internal/auth/     # Ed25519 + passphrase + JWT + refresh + devices
    internal/omp/      # stdio bridge to the omp binary
    internal/storage/  # SQLite + migrations
    internal/ssh/      # SSH keys + servers
  web/                 # Next.js front
  harness/             # roc-harness umbrella launcher
packages/
  contract/            # shared endpoints types
installer/
  install.sh           # curl | sh bootstrap
docs/
  deploy/              # Caddyfile, Cloudflare Tunnel, security checklist
scripts/
  ci.sh                # CI gate (runs on every PR)
  build-harness.sh     # cross-compile the harness
```

## Development

```bash
# Install everything.
pnpm install

# Run the gate.
bash scripts/ci.sh

# Run the api alone.
cd apps/api && go run ./cmd/api --port 30179

# Run the web alone.
cd apps/web && pnpm dev
```

## Remote deployment

See `docs/deploy/`:

- `Caddyfile` — production reverse proxy with TLS and HSTS.
- `cloudflare-tunnel.md` — simpler alternative (no inbound ports,
  automatic certs).
- `security-checklist.md` — production-readiness sign-off.

## Architecture decisions

`docs/experiments/` is the source of truth for the design decisions
behind the product. Each ADR under `docs/adr/` documents a single
decision with context, options, and consequences.

## License

MIT.
