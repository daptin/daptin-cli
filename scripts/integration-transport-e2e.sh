#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

CLI_BIN="${DAPTIN_CLI:-$TMP_DIR/daptin-cli}"
if [ -z "${DAPTIN_CLI:-}" ]; then
  (cd "$ROOT_DIR" && go build -mod=vendor -o "$CLI_BIN" .)
fi

export DAPTIN_CLI="$CLI_BIN"
export DAPTIN_CLI_REPO="$ROOT_DIR"

cd "$ROOT_DIR/scripts/transport-e2e"
go run .
