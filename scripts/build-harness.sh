#!/usr/bin/env bash
# Cross-compile the roc-harness binary for mac/linux × amd64/arm64.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="$REPO_ROOT/apps/harness/bin"
mkdir -p "$OUT_DIR"

PLATFORMS=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
)

LDFLAGS="-s -w"

cd "$REPO_ROOT/apps/harness"

for p in "${PLATFORMS[@]}"; do
  GOOS=$(echo "$p" | cut -d' ' -f1)
  GOARCH=$(echo "$p" | cut -d' ' -f2)
  EXT=""
  if [ "$GOOS" = "windows" ]; then
    EXT=".exe"
  fi
  OUT="$OUT_DIR/roc-harness-${GOOS}-${GOARCH}${EXT}"
  echo ">> $OUT"
  GOOS=$GOOS GOARCH=$GOARCH \
    go build -trimpath -ldflags="$LDFLAGS" \
    -o "$OUT" \
    ./cmd/harness
done

ls -la "$OUT_DIR"
