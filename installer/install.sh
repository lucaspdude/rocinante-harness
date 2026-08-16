#!/usr/bin/env bash
# roc-harness installer —
#   Downloads the latest API + harness binaries from the latest
#   release of lucaspdude/rocinante-harness into
#   ${ROCINANTE_SHARE_DIR}/bin, then runs `api init` to create the
#   passphrase-wrapped key.
#
# Override the version via ROCINANTE_VERSION (e.g. v0.1.1) to pin
# to a specific release. Set ROCINANTE_SKIP_INIT=1 to skip init.
# Set ROCINANTE_REPO=owner/name to install from a fork.
#
# All variables honour both ROCINANTE_* (preferred) and ROCHASSEN_*
# (legacy alias from the early alpha) — the first one set wins.
set -euo pipefail

# pick NAME [ALIASES...] — sets NAME to the first non-empty value
# among the supplied *env-var* names, defaulting to NAME=default
# when no alias is set.
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

# SHARE_DIR accepts both ROCINANTE_SHARE_DIR and ROCHASSEN_SHARE_DIR.
if [ -n "${ROCINANTE_SHARE_DIR:-${ROCHASSEN_SHARE_DIR:-}}" ]; then
  SHARE_DIR="${ROCINANTE_SHARE_DIR:-${ROCHASSEN_SHARE_DIR:-}}"
else
  SHARE_DIR="$HOME/.local/share/rocinante-harness"
fi
mkdir -p "$SHARE_DIR/bin"

# Detect skip-init flag (either prefix); default 0.
SKIP_INIT="0"
if pick SKIP_INIT_VAL ROCINANTE_SKIP_INIT ROCHASSEN_SKIP_INIT; then
  SKIP_INIT="$SKIP_INIT_VAL"
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

# Symlink the platform-suffixed binaries to the canonical names
# (api, roc-harness) so the rest of the workflow can run them
# without knowing the GOOS/GOARCH tuple.
ln -sf "$SHARE_DIR/bin/$API_NAME" "$SHARE_DIR/bin/api"
ln -sf "$SHARE_DIR/bin/$HARNESS_NAME" "$SHARE_DIR/bin/roc-harness"

echo "installed to $SHARE_DIR/bin"
ls -la "$SHARE_DIR/bin"

if [ "$SKIP_INIT" = "1" ]; then
  echo "skip-init=1; skipping init"
  exit 0
fi

"$SHARE_DIR/bin/api" --share-dir "$SHARE_DIR" init

echo "done."
echo "share-dir: $SHARE_DIR"
echo "next: run 'roc-harness up' from $SHARE_DIR/bin"
