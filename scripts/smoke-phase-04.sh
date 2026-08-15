#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

mkdir -p bin
(cd apps/api && go build -o ../../bin/api ./cmd/api)

SHARE="/tmp/roc-test/share-$$"
mkdir -p "$SHARE"
trap 'rm -rf "$SHARE"' EXIT

# 1. Init with passphrase.
echo -e 'pass\npass' | ./bin/api --share-dir "$SHARE" init
test -f "$SHARE/.ed25519"
test -f "$SHARE/roc-harness.db"

# 2. Body of .ed25519 should have kdf=argon2id.
grep -q '"kdf": "argon2id"' "$SHARE/.ed25519"

# 3. Tables in SQLite.
TABLES=$(sqlite3 "$SHARE/roc-harness.db" \
  'select count(*) from sqlite_master where type="table"' 2>/dev/null || echo 0)
if [ "$TABLES" -lt 5 ]; then
  echo "expected >=5 tables, got $TABLES"
  exit 1
fi

# 4. --no-encryption produces plaintext kdf.
PLAIN="/tmp/roc-test/plain-$$"
mkdir -p "$PLAIN"
./bin/api --share-dir "$PLAIN" --no-encryption init
test -f "$PLAIN/.ed25519"
grep -q '"kdf": "none"' "$PLAIN/.ed25519"

# 5. ROCHASSEN_SHARE_DIR honored.
ENV_DIR="/tmp/roc-test/env-$$"
mkdir -p "$ENV_DIR"
echo -e 'word\nword' | ROCHASSEN_SHARE_DIR="$ENV_DIR" ./bin/api init
test -f "$ENV_DIR/.ed25519"

echo "phase-04 smoke OK"
