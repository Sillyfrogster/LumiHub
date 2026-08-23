#!/usr/bin/env bash

set -euo pipefail

OPS_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$OPS_DIR/lib.sh"

ILLARIN_VERSION="${VERSION:-}"
export ILLARIN_VERSION
require_release
take_operation_lock

if [[ ! -d "$ILLARIN_DATA_DIR" ]]; then
  echo "The data directory does not exist: $ILLARIN_DATA_DIR" >&2
  exit 1
fi

mode="$(stat -c '%a' "$ILLARIN_ENV_FILE")"
if [[ "$mode" != "600" && "$mode" != "640" ]]; then
  echo "$ILLARIN_ENV_FILE must have mode 600 or 640; it is $mode." >&2
  exit 1
fi

minimum_free_mb="${DEPLOY_MIN_FREE_MB:-2048}"
if [[ ! "$minimum_free_mb" =~ ^[0-9]+$ ]]; then
  echo "DEPLOY_MIN_FREE_MB must be a whole number." >&2
  exit 1
fi
available_kb="$(df -Pk "$ILLARIN_DATA_DIR" | awk 'NR == 2 {print $4}')"
if (( available_kb < minimum_free_mb * 1024 )); then
  echo "Deployment stopped: less than ${minimum_free_mb} MB is free on the data disk." >&2
  exit 1
fi

current=""
if [[ -f "$ILLARIN_DATA_DIR/releases/current" ]]; then
  current="$(<"$ILLARIN_DATA_DIR/releases/current")"
fi

echo "Pulling Illarin $ILLARIN_VERSION."
compose pull api web
compose up -d --wait --wait-timeout 180 db datadog

if [[ "${BACKUPS_ENABLED:-false}" == "true" && -n "$current" ]]; then
  echo "Taking the pre-migration backup."
  compose --profile tools run --rm backup run
else
  echo "Pre-migration backup skipped because off-box backups are disabled or this is the first release."
fi

echo "Applying database migrations."
compose --profile tools run --rm migrate

rollback_after_failure() {
  if [[ ! "$current" =~ ^[0-9a-f]{40}$ ]]; then
    echo "The new release failed and there is no earlier release to restore." >&2
    return 1
  fi

  echo "The new release failed its checks. Restoring application release $current." >&2
  ILLARIN_VERSION="$current"
  export ILLARIN_VERSION
  compose up -d --wait --wait-timeout 180 api web gateway datadog
}

echo "Starting the new application containers."
if ! compose up -d --wait --wait-timeout 180 --remove-orphans api web gateway datadog; then
  rollback_after_failure || true
  exit 1
fi

if ! "$OPS_DIR/smoke.sh"; then
  rollback_after_failure || true
  exit 1
fi

if [[ -n "$current" && "$current" != "$ILLARIN_VERSION" ]]; then
  write_release previous "$current"
fi
write_release current "$ILLARIN_VERSION"

echo "Illarin $ILLARIN_VERSION is healthy."
