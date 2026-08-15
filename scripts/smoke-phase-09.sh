#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# 1. Parity test passes.
(cd apps/web && pnpm exec vitest run tests/i18n-parity.test.mjs)

# 2. Settings page renders.
(cd apps/web && nohup pnpm dev --turbopack >/tmp/p9-web.log 2>&1 &)
sleep 5
trap 'pkill -f "next dev" 2>/dev/null || true; pkill -f "next-server" 2>/dev/null || true' EXIT

for i in $(seq 1 30); do
  CODE=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:30178/ 2>/dev/null || true)
  if [ "$CODE" = "307" ] || [ "$CODE" = "200" ]; then
    break
  fi
  sleep 1
done

# 3. Settings page renders translated copy.
curl -sfL http://127.0.0.1:30178/en-US/settings | grep -q 'Settings'
curl -sfL http://127.0.0.1:30178/pt-BR/settings | grep -q 'Configurações'

echo "phase-09 smoke OK"
