#!/usr/bin/env bash
set -euo pipefail

TARGET="${1:-85}"
TMP_DIR="$(mktemp -d)"
PKG_FILE="$TMP_DIR/packages.txt"
COVER_FILE="$TMP_DIR/domain-usecase.out"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

go list ./internal/modules/... | grep -E '/(domain|usecase)$' > "$PKG_FILE"
if [[ ! -s "$PKG_FILE" ]]; then
  echo "no domain/usecase packages found" >&2
  exit 1
fi

xargs go test -coverprofile="$COVER_FILE" < "$PKG_FILE" >/dev/null
TOTAL="$(go tool cover -func="$COVER_FILE" | awk '/^total:/{gsub("%", "", $3); print $3}')"

echo "domain+usecase coverage: ${TOTAL}% (target: ${TARGET}%)"
awk -v total="$TOTAL" -v target="$TARGET" 'BEGIN { exit !(total + 0 >= target + 0) }'
