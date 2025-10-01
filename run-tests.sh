#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOCACHE_DIR="${GOCACHE:-$SCRIPT_DIR/.gocache}"
export GOCACHE="$GOCACHE_DIR"
export BLACKBOX_MIGRATIONS_DIR="${BLACKBOX_MIGRATIONS_DIR:-$SCRIPT_DIR/migrations}"

mkdir -p "$GOCACHE_DIR"

cd "$SCRIPT_DIR"

echo "==> go test ./..."
go test ./...
