#!/usr/bin/env bash
# roc-harness installer —
#   Downloads the latest API + harness binaries from the latest
#   release of lucaspdude/rocinante-harness into
#   ${ROCHASSEN_SHARE_DIR}/bin, then runs `api init` to create the
#   passphrase-wrapped key.
#
# Override the version via ROCHASSEN_VERSION (e.g. v0.1.1) to pin
# to a specific release. Set ROCHASSEN_SKIP_INIT=1 to skip init.
# Set ROCHASSEN_REPO=owner/name to install from a fork.
set -euo pipefail

REPO="${ROCHASSEN_REPO:-lucaspdude/rocinante-harness}"

# Resolve the version: explicit env > latest release tag > main.
resolve_version() {
  if [ -n "${ROCHASSEN_VERSION:-}" ]; then
    echo "$ROCHASSEN_VERSION"
    return 0
  fi
  local tag
  tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
    | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
  if [ -n "$tag" ]; then
    echo "$tag"
    return 0
  fi
  echo "main"
}
VERSION="$(resolve_version)"
echo ">> installing roc-harness ${VERSION} from ${REPO}"

ARCH=$(uname -m)
case "$ARCH" in
  x86_64) GOARCH=amd64 ;;
  arm64|aarch64) GOARCH=arm64 ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac
GOOS=$(uname | tr '[:upper:]' '[:lower:]')

SHARE_DIR="${ROCHASSEN_SHARE_DIR:-$HOME/.local/share/rocinante-harness}"
mkdir -p "$SHARE_DIR/bin"

# For real releases, download from the release assets. For
# ROCHASSEN_VERSION=main, fetch the workflow build artifacts via
# the latest CI run on main.
if [ "$VERSION" = "main" ]; then
  echo ">> ROCHASSEN_VERSION=main — fetching the latest CI build"
  RUN_ID=$(curl -fsSL "https://api.github.com/repos/${REPO}/actions/workflows/ci.yml/runs?branch=main&status=success&per_page=1" \
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

if [ "${ROCHASSEN_SKIP_INIT:-0}" = "1" ]; then
  echo "ROCHASSEN_SKIP_INIT=1; skipping init"
  exit 0
fi

"$SHARE_DIR/bin/api" --share-dir "$SHARE_DIR" init

echo "done."
echo "share-dir: $SHARE_DIR"
echo "next: run 'roc-harness up' from $SHARE_DIR/bin"
