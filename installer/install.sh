#!/usr/bin/env bash
# roc-harness installer —
#   Downloads the latest API + harness binaries + web bundle
#   from the latest release of lucaspdude/rocinante-harness into
#   ${ROCINANTE_SHARE_DIR}/, runs `api init` to create the
#   passphrase-wrapped key, optionally installs the `omp` binary,
#   and optionally installs + enables systemd units for the api
#   and the web.
#
# All flags are env vars (any prefix `ROCHASSEN_*` is a legacy
# alias for the matching `ROCINANTE_*`):
#   ROCINANTE_VERSION=v0.1.5       pin to a specific release
#   ROCINANTE_SKIP_INIT=1         skip the `api init` prompt
#   ROCINANTE_SKIP_OMP=1          skip the omp install
#   ROCINANTE_SKIP_WEB=1          skip the web bundle download
#   ROCINANTE_INSTALL_SERVICE=1   write + enable systemd units
#   ROCINANTE_BIND=0.0.0.0         bind address (default 127.0.0.1)
#   ROCINANTE_PUBLIC_API_URL=...  full URL the browser hits
#   ROCINANTE_REPO=owner/name      install from a fork
set -euo pipefail

# pick NAME [VAR_NAME...] — sets NAME to the first non-empty
# value among the supplied env-var names.
pick() {
  local name="$1"
  shift
  local alias v
  for alias in "$@"; do
    v="${!alias:-}"
    if [ -n "$v" ]; then
      printf -v "$name" '%s' "$v"
      return 0
    fi
  done
  return 1
}

pick REPO ROCINANTE_REPO ROCHASSEN_REPO \
  || REPO="lucaspdude/rocinante-harness"

# Resolve the version: explicit env > latest release tag > main.
resolve_version() {
  if pick VERSION ROCINANTE_VERSION ROCHASSEN_VERSION; then
    return 0
  fi
  local api="https://api.github.com/repos/${REPO}/releases/latest"
  local tag
  if command -v curl >/dev/null; then
    tag=$(curl -fsSL "$api" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
  else
    tag=$(wget -qO- "$api" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
  fi
  if [ -n "$tag" ]; then
    VERSION="$tag"
    return 0
  fi
  VERSION="main"
}
resolve_version
echo ">> installing roc-harness ${VERSION} from ${REPO}"

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)
    GOARCH=amd64        # Go + roc-harness asset naming
    OMP_ARCH=x64        # can1357/oh-my-pi asset naming
    ;;
  arm64|aarch64)
    GOARCH=arm64
    OMP_ARCH=arm64
    ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac
GOOS=$(uname | tr '[:upper:]' '[:lower:]')

if [ -n "${ROCINANTE_SHARE_DIR:-${ROCHASSEN_SHARE_DIR:-}}" ]; then
  SHARE_DIR="${ROCINANTE_SHARE_DIR:-${ROCHASSEN_SHARE_DIR:-}}"
else
  SHARE_DIR="$HOME/.local/share/rocinante-harness"
fi
mkdir -p "$SHARE_DIR/bin" "$SHARE_DIR/web"

SKIP_INIT="0"
if pick SKIP_INIT ROCINANTE_SKIP_INIT ROCHASSEN_SKIP_INIT; then
  SKIP_INIT="$SKIP_INIT"
fi

SKIP_OMP="0"
if pick SKIP_OMP ROCINANTE_SKIP_OMP ROCHASSEN_SKIP_OMP; then
  SKIP_OMP="$SKIP_OMP"
fi

SKIP_WEB="0"
if pick SKIP_WEB ROCINANTE_SKIP_WEB ROCHASSEN_SKIP_WEB; then
  SKIP_WEB="$SKIP_WEB"
fi

INSTALL_SERVICE="0"
if pick INSTALL_SERVICE ROCINANTE_INSTALL_SERVICE ROCHASSEN_INSTALL_SERVICE; then
  INSTALL_SERVICE="$INSTALL_SERVICE"
fi

BIND="${ROCINANTE_BIND:-127.0.0.1}"
PUBLIC_API_URL="${ROCINANTE_PUBLIC_API_URL:-}"

URL_BASE="https://github.com/${REPO}/releases/download/${VERSION}"

download() {
  local name="$1"
  local url="$URL_BASE/${name}"
  local out="$SHARE_DIR/bin/${name}"
  echo ">> fetching $name"
  if command -v curl >/dev/null; then
    curl -fL --retry 3 -o "$out" "$url"
  else
    wget -q --tries=3 -O "$out" "$url"
  fi
  chmod +x "$out"
}

API_NAME="api-${GOOS}-${GOARCH}"
HARNESS_NAME="roc-harness-${GOOS}-${GOARCH}"
download "$API_NAME"
download "$HARNESS_NAME"

ln -sf "$SHARE_DIR/bin/$API_NAME" "$SHARE_DIR/bin/api"
ln -sf "$SHARE_DIR/bin/$HARNESS_NAME" "$SHARE_DIR/bin/roc-harness"

echo "installed to $SHARE_DIR/bin"
ls -la "$SHARE_DIR/bin"

# --- web bundle (Next standalone) -------------------------------
# The web bundle is a single tar.gz named `web.tar.gz`. Release
# workflow emits it from the `next build` output (output:
# 'standalone'). The launcher extracts the bundle into
# $SHARE_DIR/web so the harness can find $SHARE_DIR/web/apps/web/server.js.
install_web() {
  if [ "$SKIP_WEB" = "1" ]; then
    echo ">> ROCINANTE_SKIP_WEB=1; skipping web bundle"
    return 0
  fi
  if [ -f "$SHARE_DIR/web/apps/web/server.js" ]; then
    echo ">> web already installed at $SHARE_DIR/web"
    return 0
  fi
  local bundle="$URL_BASE/web.tar.gz"
  local tmp="$SHARE_DIR/.web.tar.gz.tmp"
  echo ">> fetching web bundle"
  if ! curl -fL --retry 3 -o "$tmp" "$bundle"; then
    rm -f "$tmp"
    echo "warning: web bundle download failed; the web will be missing"
    return 1
  fi
  if [ ! -s "$tmp" ]; then
    rm -f "$tmp"
    echo "warning: web bundle empty"
    return 1
  fi
  if ! tar -xzf "$tmp" -C "$SHARE_DIR/web" 2>/dev/null; then
    rm -f "$tmp"
    echo "warning: web bundle extract failed"
    return 1
  fi
  rm -f "$tmp"
  if [ ! -f "$SHARE_DIR/web/apps/web/server.js" ]; then
    echo "warning: web bundle did not contain apps/web/server.js"
    return 1
  fi
  echo ">> web installed to $SHARE_DIR/web"
  return 0
}
install_web || true

# --- omp install (optional, best-effort) ---------------------------
install_omp() {
  if command -v omp >/dev/null 2>&1; then
    echo ">> omp already on PATH: $(command -v omp)"
    return 0
  fi
  if [ "$SKIP_OMP" = "1" ]; then
    echo ">> ROCINANTE_SKIP_OMP=1; leaving omp install for the user"
    return 0
  fi

  local tag
  if [ -n "${ROCINANTE_VERSION:-}" ]; then
    tag="${ROCINANTE_VERSION#v}"
  else
    local api="https://api.github.com/repos/can1357/oh-my-pi/releases/latest"
    if command -v curl >/dev/null; then
      tag=$(curl -fsSL "$api" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/')
    else
      tag=$(wget -qO- "$api" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/')
    fi
  fi
  if [ -z "$tag" ]; then
    echo "warning: could not determine omp release tag; skipping omp install"
    return 1
  fi

  local ext=""
  case "$GOOS" in
    linux) base="omp-linux-${OMP_ARCH}" ;;
    darwin) base="omp-darwin-${OMP_ARCH}" ;;
    windows) base="omp-windows-${OMP_ARCH}.exe" ;;
    *) echo "warning: no omp asset for $GOOS"; return 1 ;;
  esac
  local omp_url="https://github.com/can1357/oh-my-pi/releases/download/v${tag}/${base}"
  local tmp_dest="$SHARE_DIR/bin/.omp.download"
  echo ">> fetching $base from can1357/oh-my-pi@$tag"
  if ! curl -fsSL -o "$tmp_dest" "$omp_url"; then
    rm -f "$tmp_dest"
    echo "warning: omp download failed; install manually or set ROCINANTE_SKIP_OMP=1"
    return 1
  fi
  if [ ! -s "$tmp_dest" ]; then
    rm -f "$tmp_dest"
    echo "warning: omp download was empty; the upstream may have changed asset names"
    return 1
  fi
  local omp_dest="$SHARE_DIR/bin/omp"
  mv "$tmp_dest" "$omp_dest"
  chmod +x "$omp_dest"
  echo ">> installed omp to $omp_dest"
  return 0
}
install_omp || true

# --- init --------------------------------------------------------
if [ "$SKIP_INIT" = "1" ]; then
  echo "skip-init=1; skipping init"
else
  "$SHARE_DIR/bin/api" --share-dir "$SHARE_DIR" init
fi

# --- systemd service install (optional) ---------------------------
# Always writes the api service. Also writes the web service when
# the web bundle is present and the user opted into service
# install. Linux only.
install_service() {
  if [ "$INSTALL_SERVICE" != "1" ]; then
    return 0
  fi
  if [ "$GOOS" != "linux" ]; then
    echo ">> service install not supported on $GOOS; skipping"
    return 0
  fi
  if ! command -v systemctl >/dev/null 2>&1; then
    echo "warning: systemctl not found; skipping service install"
    return 0
  fi

  local unit_dir="/etc/systemd/system"
  local env_dir="/etc/roc-harness"
  local env_file="$env_dir/env"
  local api_unit="$unit_dir/roc-harness-api.service"
  local web_unit="$unit_dir/roc-harness-web.service"
  mkdir -p "$unit_dir" "$env_dir"

  # The api unit always runs the binary on the chosen BIND
  # address. The passphrase line is read from the env file.
  cat > "$api_unit" <<UNIT
[Unit]
Description=roc-harness api
After=network.target

[Service]
Type=simple
WorkingDirectory=$SHARE_DIR
ExecStart=$SHARE_DIR/bin/api --bind $BIND --port 30179 --share-dir $SHARE_DIR --passphrase-env ROCINANTE_PASSPHRASE --omp-bin \$OMP_BIN
EnvironmentFile=$env_file
Restart=on-failure
RestartSec=5
StandardOutput=append:/var/log/roc-harness-api.log
StandardError=append:/var/log/roc-harness-api.log

[Install]
WantedBy=multi-user.target
UNIT

  # Seed the env file with OMP_BIN pointing at the share-dir binary.
  if [ ! -f "$env_file" ]; then
    local omp_bin_path
    omp_bin_path="$(command -v omp 2>/dev/null || true)"
    if [ -z "$omp_bin_path" ] && [ -x "$SHARE_DIR/bin/omp" ]; then
      omp_bin_path="$SHARE_DIR/bin/omp"
    fi
    {
      echo "OMP_BIN=${omp_bin_path:-}"
    } > "$env_file"
    chmod 0600 "$env_file"
    echo ">> wrote $env_file (with OMP_BIN=${omp_bin_path:-<not found>})"
  fi

  if ! grep -q '^ROCINANTE_PASSPHRASE=' "$env_file" 2>/dev/null; then
    if [ -t 0 ]; then
      printf 'ROCINANTE_PASSPHRASE=' >> "$env_file"
      stty -echo 2>/dev/null || true
      read -r -p "passphrase for the service (appended to $env_file, mode 0600): " PW
      stty echo 2>/dev/null || true
      echo
      echo >> "$env_file"
      chmod 0600 "$env_file"
      echo ">> appended passphrase to $env_file"
    else
      echo "warning: non-interactive shell; no passphrase in $env_file"
      echo "         append a line:"
      echo "           printf 'ROCINANTE_PASSPHRASE=your-passphrase' >> $env_file"
      echo "         then re-run: systemctl start roc-harness-api"
      systemctl daemon-reload
      echo ">> wrote $api_unit (service not enabled yet)"
      return 0
    fi
  fi

  systemctl daemon-reload
  systemctl enable --now roc-harness-api
  echo ">> enabled roc-harness-api"
  systemctl --no-pager status roc-harness-api | head -5

  # The web unit is written + enabled only if the standalone
  # bundle is present. The api advertises its public URL to the
  # web via the env file so the frontend can build the right
  # ^/api/v1/...^ links.
  if [ ! -f "$SHARE_DIR/web/apps/web/server.js" ]; then
    echo ">> web bundle not present; skipping roc-harness-web unit"
    return 0
  fi

  cat > "$web_unit" <<WUNIT
[Unit]
Description=roc-harness web
After=network.target roc-harness-api.service

[Service]
Type=simple
WorkingDirectory=$SHARE_DIR/web/apps/web
ExecStart=/usr/bin/node $SHARE_DIR/web/apps/web/server.js
EnvironmentFile=$env_file
Restart=on-failure
RestartSec=5
StandardOutput=append:/var/log/roc-harness-web.log
StandardError=append:/var/log/roc-harness-web.log

[Install]
WantedBy=multi-user.target
WUNIT

  # The web reads its api URL from the same env file. We always
  # set RH_API_INTERNAL_URL to the api's loopback address (the
  # server-side render runs inside the same host and the api is
  # bound to 127.0.0.1 from the server's perspective). The
  # NEXT_PUBLIC_RH_API_URL is what the browser hits; if the user
  # didn't set it explicitly, we synthesise it from the bind
  # address.
  if ! grep -q '^RH_API_INTERNAL_URL=' "$env_file"; then
    printf 'RH_API_INTERNAL_URL=http://127.0.0.1:30179\n' >> "$env_file"
  fi
  if [ -n "$PUBLIC_API_URL" ] && ! grep -q '^NEXT_PUBLIC_RH_API_URL=' "$env_file"; then
    printf 'NEXT_PUBLIC_RH_API_URL=%s\n' "$PUBLIC_API_URL" >> "$env_file"
  elif ! grep -q '^NEXT_PUBLIC_RH_API_URL=' "$env_file"; then
    local scheme="http"
    local host="$BIND"
    [ "$BIND" = "0.0.0.0" ] && host=$(hostname -I | awk '{print $1}')
    printf 'NEXT_PUBLIC_RH_API_URL=%s://%s:30179\n' "$scheme" "$host" >> "$env_file"
  fi

  systemctl daemon-reload
  systemctl enable --now roc-harness-web
  echo ">> enabled roc-harness-web"
  systemctl --no-pager status roc-harness-web | head -5
}
install_service

echo "done."
echo "share-dir: $SHARE_DIR"
echo "next: run 'roc-harness up' from $SHARE_DIR/bin"
