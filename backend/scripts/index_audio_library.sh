#!/usr/bin/env sh

set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
LIBRARY_DIR="${1:-$ROOT_DIR/data/all_datas_shuffle}"
DB_PATH="${DB_PATH:-$ROOT_DIR/data/kiria.db}"

if [ ! -d "$LIBRARY_DIR" ]; then
  echo "audio library directory not found: $LIBRARY_DIR" >&2
  exit 1
fi

if [ "${RESET_DB:-0}" = "1" ]; then
  rm -f "$DB_PATH"
fi

cd "$ROOT_DIR"
go run ./cmd/indexer -reference -skip-existing "$LIBRARY_DIR"
