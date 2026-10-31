#!/usr/bin/env bash
# Build the Unraid amd64 binary into /tmp without leaving generated assets in
# the working tree. Used after every change for continuous testing.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${AURORA_TMP_BIN:-/tmp/aurora-linux-amd64}"
cd "$ROOT"

# Preserve whatever is currently embedded so the source tree stays clean after
# the build (normally the tracked placeholder index.html).
RESTORE_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$ROOT/backend/internal/webui/dist"
  if [ -d "$RESTORE_DIR/dist" ]; then
    cp -R "$RESTORE_DIR/dist" "$ROOT/backend/internal/webui/dist"
  fi
  rm -rf "$RESTORE_DIR"
}
trap cleanup EXIT

if [ -d "$ROOT/backend/internal/webui/dist" ]; then
  cp -R "$ROOT/backend/internal/webui/dist" "$RESTORE_DIR/dist"
fi

make --no-print-directory ui

cd "$ROOT/backend"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w" -o "$OUT" ./cmd/aurorad

echo
echo "binary: $OUT"
file "$OUT"
ls -lh "$OUT"
shasum -a 256 "$OUT"
