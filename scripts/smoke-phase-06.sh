#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# 1. Parity test passes.
(cd apps/web && pnpm exec vitest run tests/i18n-parity.test.mjs)

# 2. Boot the web dev server.
(cd apps/web && nohup pnpm dev --turbopack >/tmp/p6-web.log 2>&1 &)
sleep 5
trap 'pkill -f "next dev" 2>/dev/null || true; pkill -f "next-server" 2>/dev/null || true' EXIT

# 3. Wait for the root to respond.
for i in $(seq 1 90); do
  CODE=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:30178/ 2>/dev/null || true)
  if [ "$CODE" = "307" ] || [ "$CODE" = "200" ]; then
    break
  fi
  sleep 1
done

# 4. Root page redirects to /en-US/.
curl -sI http://127.0.0.1:30178/ | grep -E '^(HTTP|location)' | head -3

# 5. /en-US/login renders the form.
curl -sfL http://127.0.0.1:30178/en-US/login | grep -q 'name="passphrase"'

# 6. /pt-BR/login renders with translated copy.
curl -sfL http://127.0.0.1:30178/pt-BR/login | grep -q 'name="passphrase"'

# 7. Translation payload differs between locales (smoke that
# the dictionaries actually route to the page).
curl -sfL http://127.0.0.1:30178/en-US/login | grep -q 'Sign in'
curl -sfL http://127.0.0.1:30178/pt-BR/login | grep -q 'Entrar'

echo "phase-06 smoke OK"
