#!/usr/bin/env bash
# roc-harness installer —
#   Downloads the latest API + harness binaries to
#   ${ROCHASSEN_SHARE_DIR}/bin, then runs `api init` to create the
#   passphrase-wrapped key. Use ROCHASSEN_SKIP_INIT=1 to skip the
#   init step (e.g. CI).
set -euo pipefail

REPO="${ROCHASSEN_REPO:-lucaspdude/rocinante-harness}"
VERSION="${ROCHASSEN_VERSION:-v0.1.0}"
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) GOARCH=amd64 ;;
  arm64|aarch64) GOARCH=arm64 ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac
GOOS=$(uname | tr '[:upper:]' '[:lower:]')

SHARE_DIR="${ROCHASSEN_SHARE_DIR:-$HOME/.local/share/rocinante-harness}"
mkdir -p "$SHARE_DIR/bin"

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

download "api-${GOOS}-${GOARCH}"
download "roc-harness-${GOOS}-${GOARCH}"

echo "installed to $SHARE_DIR/bin"

if [ "${ROCHASSEN_SKIP_INIT:-0}" = "1" ]; then
  echo "ROCHASSEN_SKIP_INIT=1; skipping init"
  exit 0
fi

# Run init.
"$SHARE_DIR/bin/api" --share-dir "$SHARE_DIR" init

echo "done."
echo "share-dir: $SHARE_DIR"
echo "next: run 'roc-harness up' from $SHARE_DIR/bin"
