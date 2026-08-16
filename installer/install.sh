#!/usr/bin/env bash
# roc-harness installer —
#   Downloads the latest API + harness binaries from the latest
#   release of lucaspdude/rocinante-harness into
#   ${ROCINANTE_SHARE_DIR}/bin, runs `api init` to create the
#   passphrase-wrapped key, optionally installs the `omp` binary
#   if missing, and optionally installs + enables a systemd
#   service for the api.
#
# All flags are env vars (any prefix `ROCHASSEN_*` is a legacy
# alias for the matching `ROCINANTE_*`):
#   ROCINANTE_VERSION=v0.1.5       pin to a specific release
#   ROCINANTE_SKIP_INIT=1         skip the `api init` prompt
#   ROCINANTE_SKIP_OMP=1          skip the omp install
#   ROCINANTE_INSTALL_SERVICE=1   write + enable systemd units
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
  x86_64) GOARCH=amd64 ;;
  arm64|aarch64) GOARCH=arm64 ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac
GOOS=$(uname | tr '[:upper:]' '[:lower:]')

if [ -n "${ROCINANTE_SHARE_DIR:-${ROCHASSEN_SHARE_DIR:-}}" ]; then
  SHARE_DIR="${ROCINANTE_SHARE_DIR:-${ROCHASSEN_SHARE_DIR:-}}"
else
  SHARE_DIR="$HOME/.local/share/rocinante-harness"
fi
mkdir -p "$SHARE_DIR/bin"

SKIP_INIT="0"
if pick SKIP_INIT ROCINANTE_SKIP_INIT ROCHASSEN_SKIP_INIT; then
  SKIP_INIT="$SKIP_INIT"
fi

SKIP_OMP="0"
if pick SKIP_OMP ROCINANTE_SKIP_OMP ROCHASSEN_SKIP_OMP; then
  SKIP_OMP="$SKIP_OMP"
fi

INSTALL_SERVICE="0"
if pick INSTALL_SERVICE ROCINANTE_INSTALL_SERVICE ROCHASSEN_INSTALL_SERVICE; then
  INSTALL_SERVICE="$INSTALL_SERVICE"
fi

if [ "$VERSION" = "main" ]; then
  echo ">> ROCINANTE_VERSION=main — fetching the latest CI build"
  RUN_ID=$(curl -fsSL "https://api.github.com/repos/${REPO}/actions/workflows/ci.yml/runs?branch=main&status=success&per_page=1" 2>/dev/null \
    | grep '"id"' | head -1 | sed -E 's/.*"([0-9]+)".*/\1/')
  if [ -z "$RUN_ID" ]; then
    echo "no successful CI run on main; aborting" >&2
    exit 1
  fi
  URL_BASE="https://github.com/${REPO}/actions/runs/${RUN_ID}"
else
  URL_BASE="https://github.com/${REPO}/releases/download/${VERSION}"
fi

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

# --- omp install (optional, best-effort) ---------------------------
# The api needs the omp binary on $PATH (or pointed at via
# --omp-bin). If omp is missing and the user has not opted out,
# try a best-effort download from the omp project's own release
# page. If the install fails we print a warning rather than
# aborting — the api boots fine without omp; the user can install
# it manually later.
install_omp() {
  if command -v omp >/dev/null 2>&1; then
    echo ">> omp already on PATH: $(command -v omp)"
    return 0
  fi
  if [ "$SKIP_OMP" = "1" ]; then
    echo ">> ROCINANTE_SKIP_OMP=1; leaving omp install for the user"
    return 0
  fi
  if [ "$GOOS" != "linux" ]; then
    echo ">> skip omp auto-install: not on linux ($GOOS)"
    return 0
  fi
  echo ">> omp not found; attempting best-effort install"
  local omp_url="https://github.com/l3yx/oh-my-pi/releases/latest/download/omp-linux-${GOARCH}"
  local omp_dest="$SHARE_DIR/bin/omp"
  if command -v curl >/dev/null; then
    if curl -fsSL -o "$omp_dest" "$omp_url" 2>/dev/null && [ -s "$omp_dest" ]; then
      chmod +x "$omp_dest"
      echo ">> installed omp to $omp_dest"
      return 0
    fi
  fi
  echo "warning: omp install failed; sessions will return spawn_failed until"
  echo "         the user installs omp and points --omp-bin at it"
  return 1
}
install_omp || true

# --- init --------------------------------------------------------
if [ "$SKIP_INIT" = "1" ]; then
  echo "skip-init=1; skipping init"
else
  "$SHARE_DIR/bin/api" --share-dir "$SHARE_DIR" init
fi

# --- systemd service install (optional) ---------------------------
# Writes a unit file that runs the api under systemd with the
# passphrase pulled from /etc/roc-harness/passphrase. On Linux
# distros that ship systemd. On macOS / Windows this is a no-op.
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
  local env_file="$env_dir/passphrase"
  local unit_file="$unit_dir/roc-harness-api.service"
  mkdir -p "$unit_dir" "$env_dir"

  # Always write the unit file. The EnvironmentFile= directive
  # only references the passphrase file and is conditional —
  # when the file is missing we leave the unit stopped and
  # instruct the user to re-run with a TTY (or to create the
  # passphrase file manually).
  cat > "$unit_file" <<UNIT
[Unit]
Description=roc-harness api
After=network.target

[Service]
Type=simple
WorkingDirectory=$SHARE_DIR
ExecStart=$SHARE_DIR/bin/api --port 30179 --share-dir $SHARE_DIR --passphrase-env ROCINANTE_PASSPHRASE
Restart=on-failure
RestartSec=5
StandardOutput=append:/var/log/roc-harness-api.log
StandardError=append:/var/log/roc-harness-api.log

[Install]
WantedBy=multi-user.target
UNIT

  # Write the passphrase file. The interactive branch uses read
  # with -t and a hidden prompt. The non-interactive branch
  # requires the user to have already set the passphrase file
  # outside this script (e.g. via cloud-init, an ops tool, or a
  # previous run). When the file is missing, we install the
  # unit but do NOT enable or start the service — let the user
  # create the file and run `systemctl start roc-harness-api`.
  if [ ! -f "$env_file" ]; then
    if [ -t 0 ]; then
      printf 'ROCINANTE_PASSPHRASE=' > "$env_file.tmp"
      stty -echo 2>/dev/null || true
      read -r -p "passphrase for the service (will be stored in $env_file, mode 0600): " PW
      stty echo 2>/dev/null || true
      echo
      echo "$PW" >> "$env_file.tmp"
      mv "$env_file.tmp" "$env_file"
      chmod 0600 "$env_file"
      echo ">> wrote $env_file"
    else
      echo "warning: non-interactive shell; no passphrase file at $env_file"
      echo "         create it with:"
      echo "           printf 'ROCINANTE_PASSPHRASE=your-passphrase' > $env_file"
      echo "           chmod 0600 $env_file"
      echo "         then re-run: systemctl start roc-harness-api"
      systemctl daemon-reload
      echo ">> wrote $unit_file (service not enabled yet)"
      return 0
    fi
  fi

  systemctl daemon-reload
  systemctl enable --now roc-harness-api
  echo ">> enabled roc-harness-api"
  systemctl --no-pager status roc-harness-api | head -5
}
install_service

echo "done."
echo "share-dir: $SHARE_DIR"
echo "next: run 'roc-harness up' from $SHARE_DIR/bin"
