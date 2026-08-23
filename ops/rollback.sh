#!/usr/bin/env bash

set -euo pipefail

OPS_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$OPS_DIR/lib.sh"

take_operation_lock

if [[ ! -f "$ILLARIN_DATA_DIR/releases/current" || ! -f "$ILLARIN_DATA_DIR/releases/previous" ]]; then
  echo "Both a current and previous release are required for rollback." >&2
  exit 1
fi

current="$(<"$ILLARIN_DATA_DIR/releases/current")"
previous="$(<"$ILLARIN_DATA_DIR/releases/previous")"
if [[ ! "$current" =~ ^[0-9a-f]{40}$ || ! "$previous" =~ ^[0-9a-f]{40}$ ]]; then
  echo "The recorded release files do not contain full Git commit SHAs." >&2
  exit 1
fi

echo "Rolling application containers back to $previous. Database migrations are not reversed."
ILLARIN_VERSION="$previous"
export ILLARIN_VERSION
compose pull api web

if ! compose up -d --wait --wait-timeout 180 api web gateway; then
  echo "Rollback failed. Attempting to restore $current." >&2
  ILLARIN_VERSION="$current"
  export ILLARIN_VERSION
  compose up -d --wait --wait-timeout 180 api web gateway || true
  exit 1
fi

if ! "$OPS_DIR/smoke.sh"; then
  echo "Rollback smoke checks failed. Attempting to restore $current." >&2
  ILLARIN_VERSION="$current"
  export ILLARIN_VERSION
  compose up -d --wait --wait-timeout 180 api web gateway || true
  exit 1
fi

write_release current "$previous"
write_release previous "$current"

echo "Illarin is running application release $previous."
