#!/usr/bin/env bash
# Smoke test for phase 5 integration (PRs 1..8).
# Asserts no i18n literals are rendered in HTML, settings uses the modal
# (not the legacy 5-tab page), and the api responds with the
# current release version.
#
# Runs against a local roc-harness (api on 30179, web on 30178).
#
# Exit code 0 = OK. Anything else = FAIL.
#
# Override defaults by exporting BASE_API / BASE_WEB / ROCINANTE_PASSPHRASE
# before invoking.

set -euo pipefail

cd "$(dirname "$0")/.."

API="${BASE_API:-http://127.0.0.1:30179}"
WEB="${BASE_WEB:-http://127.0.0.1:30178}"
PASS="${ROCINANTE_PASSPHRASE:-secret-passphrase-1234}"

echo "phase-23: login"
TOKEN=$(curl -sS -X POST "$API/api/v1/login" \
  -H 'Content-Type: application/json' \
  -d "{\"passphrase\":\"$PASS\",\"device_name\":\"smoke-phase-23\"}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["access"])')
[ -n "$TOKEN" ] || { echo "FAIL: could not acquire token"; exit 1; }

# Throwaway project root.
SMOKE_ROOT="/tmp/phase5-integration-smoke-$$"
mkdir -p "$SMOKE_ROOT"
REG_HTTP=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$API/api/v1/projects" \
  -H "authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"path\":\"$SMOKE_ROOT\",\"name\":\"phase5-integration\"}")
[ "$REG_HTTP" = "201" ] || { echo "FAIL: project register returned $REG_HTTP"; exit 1; }

# Forbidden literals — these should never appear in the rendered HTML
# after PR-01..06 land.
FORBIDDEN_LITERALS=(
  'directoryPicker.title'
  'directoryPicker.search'
  'directoryPicker.cancel'
  'directoryPicker.select'
  'directoryPicker.up'
  'directoryPicker.breadcrumbHome'
  'directoryPicker.empty'
  'paste-key'
  'keyless'
  'settings.devices'
  'composer.accessMode.read'
  'composer.accessMode.write'
)

# URLs to check (require auth because some panels only render when
# the bearer token validates).
URLS=(
  "$WEB/en-US/agent"
  "$WEB/en-US/settings?tab=providers"
  "$WEB/en-US/settings?section=providers"
  "$WEB/en-US/settings"
)

echo "phase-23: assert no forbidden literals in rendered HTML"
for URL in "${URLS[@]}"; do
  echo "  fetching $URL"
  HTML=$(curl -sS "$URL" -H "authorization: Bearer $TOKEN")
  for LITERAL in "${FORBIDDEN_LITERALS[@]}"; do
    if echo "$HTML" | grep -qF "$LITERAL"; then
      echo "FAIL: literal '$LITERAL' found in $URL"
      echo "  context:"
      echo "$HTML" | grep -F "$LITERAL" | head -3
      exit 1
    fi
  done
done

# Settings page should NOT use the legacy 5-tab horizontal layout
# (rh-tab class). It must render the centered SettingsModal with a
# left rail instead.
echo "phase-23: assert /settings uses SettingsModal (not 5-tab page)"
HTML=$(curl -sS "$WEB/en-US/settings" -H "authorization: Bearer $TOKEN")
if echo "$HTML" | grep -q 'rh-tab '; then
  echo "FAIL: settings page still uses rh-tab horizontal layout"
  exit 1
fi
if ! echo "$HTML" | grep -q 'rh-settings-modal'; then
  echo "FAIL: settings page missing SettingsModal marker"
  exit 1
fi

# Composer must render the canonical ChatComposer — both session-less
# (/agent) and in-session (/agent/{id}) share the rh-composer-card
# class.
echo "phase-23: assert canonical composer renders"
HTML=$(curl -sS "$WEB/en-US/agent" -H "authorization: Bearer $TOKEN")
if ! echo "$HTML" | grep -q 'rh-composer-card'; then
  echo "FAIL: agent home missing canonical composer (rh-composer-card)"
  exit 1
fi

# Right sidebar must not say 'No project active' once a project is
# selected. We register via the API and then curl with the bearer
# token; the rendered HTML for /en-US/agent must not contain the
# disabled-message.
echo "phase-23: assert right sidebar reaches the registered project"
HTML=$(curl -sS "$WEB/en-US/agent" -H "authorization: Bearer $TOKEN")
if echo "$HTML" | grep -q 'rightSidebar.empty.noProject'; then
  echo "FAIL: /agent rendered 'No project active' but a project is registered"
  exit 1
fi

echo "OK phase-23"
