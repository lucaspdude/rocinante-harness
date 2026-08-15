# Remote deployment security checklist

Before exposing roc-harness on a public hostname, every item on
this list must be green.

## TLS

- [ ] TLS ≥ 1.2 on every public endpoint (api.example.com and
      app.example.com).
- [ ] Certificate is renewable (Caddy + Let's Encrypt or
      Cloudflare) — no self-signed certs in production.
- [ ] HSTS header present (`Strict-Transport-Security:
      max-age=15552000; includeSubDomains`).
- [ ] HTTP→HTTPS redirect enabled at the edge.

## Authentication

- [ ] `api init` ran on the VPS with a **strong** passphrase (≥ 16
      characters, random).
- [ ] `--passphrase-env` is set in the service unit so the api
      can unwrap the key without prompting.
- [ ] `--no-encryption` is **not** set in production.
- [ ] The .ed25519.bak file lives somewhere other than the
      share-dir (1Password, age-encrypted offsite, etc.).

## CORS

- [ ] `--cors-allowlist` contains only the front's origin (e.g.
      `https://app.example.com`). No wildcard, no empty.
- [ ] The api logs an explicit warning when `--bind 0.0.0.0`
      is set without an allow-list.

## Rate limiting

- [ ] Login: 10/min per IP, persistent in SQLite (migration 003).
- [ ] Refresh: 60/min per device.
- [ ] Pairing: 30/h per device or IP.

## Process

- [ ] roc-harness runs as a non-root user (`roc-harness`).
- [ ] systemd service unit has `NoNewPrivileges=true`,
      `ProtectSystem=strict`, `ProtectHome=yes`,
      `PrivateTmp=true`.
- [ ] Logs rotate (`logrotate` or `journald`).
- [ ] Backups: .ed25519.bak exported offsite every 24h.

## Operating

- [ ] `roc-harness status` exits 0.
- [ ] `api /api/v1/onboarding/status` returns
      `initialized: true`.
- [ ] Smoke test on a freshly-built VPS reproduces the
      expected flow in < 5 minutes.
