#!/usr/bin/env bash
# roc-harness installer.
#
# One-shot setup: downloads the api, harness, and web bundle from
# the latest release of lucaspdude/rocinante-harness, writes +
# enables systemd units, and starts the api + web as sibling
# processes.
#
# The web talks to the api via a Next.js rewrite
# (/api/v1/* → 127.0.0.1:30179/api/v1/*, see apps/web/next.config.ts).
# The browser always talks to the web on :30178 — same origin, no
# CORS, no public-host / bind-address env var to remember.
#
# Usage:
#   curl -fsSL .../installer/install.sh | bash
#
# Optional env vars (most users will not need any):
#   ROCINANTE_VERSION=v0.1.5   pin a release (default: latest)
#   ROCINANTE_REPO=owner/name   install from a fork
#   ROCINANTE_SHARE_DIR=/opt/rh override the install root
#                          (default: $HOME/.local/share/rocinante-harness)
#   ROCINANTE_PASSPHRASE=...   non-interactive init (otherwise the
#                              installer prompts via /dev/tty)
#
# Removed env vars (no longer honored as of v1.16.4, PR-08):
#   OMP_WEB_THEME=warm-paper|warm-ember|...
#                              The web picks theme via Settings →
#                              Appearance (Light / Dark / System),
#                              persisted in localStorage["rh-theme"]
#                              and exposed via the HTML color-scheme
#                              attribute. Historical OMP_WEB_THEME
#                              values are mapped to the modern
#                              preference space by useTheme() for
#                              one boot cycle.
#
# What gets written:
#   $SHARE_DIR/bin/{api,roc-harness,omp}    binaries
#   $SHARE_DIR/web/apps/web/                 web bundle
#   $SHARE_DIR/.ed25519(.bak)                api key (created by `api init`)
#   $SHARE_DIR/roc-harness.db                api state
#   /etc/roc-harness/env                     api env (passphrase + omp path)
#   /etc/systemd/system/roc-harness-{api,web}.service
set -euo pipefail

REPO="${ROCINANTE_REPO:-lucaspdude/rocinante-harness}"

# Resolve the version: explicit env > latest release tag.
VERSION="${ROCINANTE_VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/')
fi
if [ -z "$VERSION" ]; then
  echo "could not resolve the latest release tag from ${REPO}" >&2
  exit 1
fi
VERSION="v${VERSION#v}"
echo ">> installing roc-harness ${VERSION} from ${REPO}"

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  GOARCH=amd64;  OMP_ARCH=x64   ;;
  arm64|aarch64) GOARCH=arm64;  OMP_ARCH=arm64 ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac
GOOS=$(uname | tr '[:upper:]' '[:lower:]')

SHARE_DIR="${ROCINANTE_SHARE_DIR:-$HOME/.local/share/rocinante-harness}"
mkdir -p "$SHARE_DIR/bin" "$SHARE_DIR/web"

URL_BASE="https://github.com/${REPO}/releases/download/${VERSION}"

fetch() {
  local name="$1"
  local url="$URL_BASE/${name}"
  local out="$SHARE_DIR/bin/${name}"
  echo ">> fetching $name"
  curl -fL --retry 3 -o "$out.tmp" "$url"
  mv "$out.tmp" "$out"
  chmod +x "$out"
}

API_NAME="api-${GOOS}-${GOARCH}"
HARNESS_NAME="roc-harness-${GOOS}-${GOARCH}"
fetch "$API_NAME"
fetch "$HARNESS_NAME"

ln -sf "$SHARE_DIR/bin/$API_NAME"    "$SHARE_DIR/bin/api"
ln -sf "$SHARE_DIR/bin/$HARNESS_NAME" "$SHARE_DIR/bin/roc-harness"
echo "installed to $SHARE_DIR/bin"
ls -la "$SHARE_DIR/bin"

# --- web bundle (Next standalone) ------------------------------------
# Always re-download on every install invocation. The web bundle is
# owned by the release pipeline — leaving a stale bundle in place when
# upgrading strands the user on a previous release's UI (bit us on
# the v1.22.0 -> v1.23.0 upgrade where the i18n bundle was missing
# keys). Wipe the existing bundle before extracting so stale files
# from a previous version are not kept.
echo ">> fetching web bundle"
curl -fL --retry 3 -o "$SHARE_DIR/.web.tar.gz.tmp" "$URL_BASE/web.tar.gz"
rm -rf "$SHARE_DIR/web/apps"
mkdir -p "$SHARE_DIR/web"
tar -xzf "$SHARE_DIR/.web.tar.gz.tmp" -C "$SHARE_DIR/web"
rm -f "$SHARE_DIR/.web.tar.gz.tmp"
if [ ! -f "$SHARE_DIR/web/apps/web/server.js" ]; then
  echo "fatal: web bundle did not contain apps/web/server.js" >&2
  exit 1
fi
echo ">> web installed to $SHARE_DIR/web"

# --- omp install (best-effort) ---------------------------------------
if ! command -v omp >/dev/null 2>&1 && [ ! -x "$SHARE_DIR/bin/omp" ]; then
  echo ">> fetching omp"
  OMP_BASE=""
  case "$GOOS" in
    linux)   OMP_BASE="omp-linux-${OMP_ARCH}"  ;;
    darwin)  OMP_BASE="omp-darwin-${OMP_ARCH}" ;;
    windows) OMP_BASE="omp-windows-${OMP_ARCH}.exe" ;;
  esac
  OMP_URL="https://github.com/can1357/oh-my-pi/releases/latest/download/${OMP_BASE}"
  if curl -fsSL -o "$SHARE_DIR/bin/omp.tmp" "$OMP_URL"; then
    mv "$SHARE_DIR/bin/omp.tmp" "$SHARE_DIR/bin/omp"
    chmod +x "$SHARE_DIR/bin/omp"
    echo ">> installed omp to $SHARE_DIR/bin/omp"
  else
    rm -f "$SHARE_DIR/bin/omp.tmp"
    echo "warning: omp download failed; install manually"
  fi
fi

# --- api init --------------------------------------------------------
if [ -f "$SHARE_DIR/.ed25519" ]; then
  echo ">> .ed25519 already exists at $SHARE_DIR/.ed25519; skipping init"
else
  echo ">> running api init"
  "$SHARE_DIR/bin/api" --share-dir "$SHARE_DIR" init
fi

# --- systemd units ---------------------------------------------------
if [ "$GOOS" = "linux" ] && command -v systemctl >/dev/null 2>&1; then
  ENV_FILE="/etc/roc-harness/env"
  mkdir -p /etc/roc-harness

  # Find the node binary. /usr/bin/node is the common location on
  # Debian/Ubuntu; nvm puts it under ~/.nvm/versions/node/v*/bin/node.
  # Falling back to PATH would not help systemd (it ignores the
  # interactive-shell PATH). We resolve it at install time.
  NODE_BIN="$(command -v node)"
  if [ -z "$NODE_BIN" ]; then
    for candidate in /usr/bin/node /usr/local/bin/node /opt/node/bin/node; do
      [ -x "$candidate" ] && NODE_BIN="$candidate" && break
    done
  fi
  if [ -z "$NODE_BIN" ]; then
    echo "fatal: could not find a node binary in PATH or /usr/bin/node" >&2
    echo "       install node first (apt install nodejs, brew install node, etc.)" >&2
    exit 1
  fi
  echo ">> node binary: $NODE_BIN"

  # Seed /etc/roc-harness/env (don't overwrite an existing file;
  # only add the keys that aren't already there).
  if [ ! -f "$ENV_FILE" ]; then
    {
      echo "ROCINANTE_PASSPHRASE=${ROCINANTE_PASSPHRASE:-}"
      echo "OMP_BIN=$SHARE_DIR/bin/omp"
      echo "PORT=30178"
    } > "$ENV_FILE"
    chmod 0600 "$ENV_FILE"
  else
    grep -q '^ROCINANTE_PASSPHRASE=' "$ENV_FILE" || \
      [ -n "${ROCINANTE_PASSPHRASE:-}" ] && \
      echo "ROCINANTE_PASSPHRASE=$ROCINANTE_PASSPHRASE" >> "$ENV_FILE"
    grep -q '^OMP_BIN='              "$ENV_FILE" || echo "OMP_BIN=$SHARE_DIR/bin/omp" >> "$ENV_FILE"
    grep -q '^PORT='                "$ENV_FILE" || echo "PORT=30178"               >> "$ENV_FILE"
  fi

  # Forward known provider keys from the workspace's .env.local
  # (if it exists) into the api's EnvironmentFile. The api
  # never reads the key itself; it just inherits os.Environ() to
  # the omp subprocess. We only forward known provider names to
  # avoid leaking arbitrary secrets into a chmod 0600 root-owned
  if [ -f "$SHARE_DIR/../../.env.local" ]; then
    while IFS= read -r line; do
      case "$line" in
        ''|\#*) continue ;;           # skip blanks/comments
        ANTHROPIC_API_KEY=*|OPENAI_API_KEY=*|GEMINI_API_KEY=*|OPENROUTER_API_KEY=*|MINIMAX_API_KEY=*)
          name="${line%%=*}"
          if ! grep -q "^${name}=" "$ENV_FILE" 2>/dev/null; then
            echo ">> forwarding $name from .env.local"
            echo "$line" >> "$ENV_FILE"
          fi
          ;;
        *)
          echo ">> ignoring $SHARE_DIR/../../.env.local line: $line" >&2
          ;;
      esac
    done < "$SHARE_DIR/../../.env.local"
  fi
  # No ProtectHome: the install path defaults to $HOME/.local/share,
  # which lives under /root when run as root and systemd's
  # ProtectHome=yes would refuse to exec /root/.local.
  cat > /etc/systemd/system/roc-harness-api.service <<UNIT
[Unit]
Description=roc-harness api
After=network.target

[Service]
Type=simple
WorkingDirectory=$SHARE_DIR
ExecStart=$SHARE_DIR/bin/api --share-dir $SHARE_DIR --port 30179 --bind ${ROCINANTE_HOST:-127.0.0.1}
EnvironmentFile=$ENV_FILE
# systemd does not export HOME by default; envconfig.ShareDir()
# falls through to os.UserHomeDir() and warns "$HOME is not defined"
# when it is unset. Pin HOME=/root so ShareDir() resolves to
# /root/.local/share/rocinante-harness — where the installer
# already wrote .ed25519 + roc-harness.db.
Environment=HOME=/root
Restart=always
RestartSec=5
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
UNIT

  cat > /etc/systemd/system/roc-harness-web.service <<WUNIT
[Unit]
Description=roc-harness web
After=network.target roc-harness-api.service

[Service]
Type=simple
WorkingDirectory=$SHARE_DIR/web/apps/web
ExecStart=$NODE_BIN $SHARE_DIR/web/apps/web/server.js
EnvironmentFile=$ENV_FILE
Restart=on-failure
RestartSec=5
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
WUNIT

  systemctl daemon-reload
  systemctl enable --now roc-harness-api roc-harness-web
  echo ">> enabled + started roc-harness-api and roc-harness-web"
  systemctl --no-pager status roc-harness-api roc-harness-web | head -10
else
  echo ">> service install skipped (not linux or no systemctl)"
  echo ">> to start manually:"
  echo "     $SHARE_DIR/bin/api --share-dir $SHARE_DIR --port 30179 &"
  echo "     cd $SHARE_DIR/web/apps/web && PORT=30178 node server.js &"
fi

echo
echo "done."
echo "share-dir: $SHARE_DIR"
echo "open:      http://localhost:30178  (or http://<host-ip>:30178)"
