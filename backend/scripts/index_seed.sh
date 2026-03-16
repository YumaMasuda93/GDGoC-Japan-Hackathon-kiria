#!/usr/bin/env sh

set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SEED_DIR="${1:-$ROOT_DIR/seed}"

if [ ! -d "$SEED_DIR" ]; then
  echo "seed directory not found: $SEED_DIR" >&2
  exit 1
fi

find "$SEED_DIR" -type f \
  \( -name '*.wav' -o -name '*.mp3' -o -name '*.m4a' -o -name '*.ogg' -o -name '*.flac' \) \
  -exec go run ./cmd/indexer {} +
