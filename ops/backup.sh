#!/usr/bin/env bash

set -euo pipefail

OPS_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$OPS_DIR/lib.sh"

require_release

if [[ "${BACKUPS_ENABLED:-false}" != "true" ]]; then
  echo "Off-box backups are disabled. See ops/README.md before enabling them." >&2
  exit 1
fi

action="${1:-run}"
if [[ "$action" != "run" && "$action" != "init" && "$action" != "check" ]]; then
  echo "Usage: backup.sh [run|init|check]" >&2
  exit 1
fi

take_operation_lock
started_at="$(date +%s)"

report_result() {
  local status="$?"
  local success=0
  local finished_at
  local duration

  trap - EXIT
  if (( status == 0 )); then
    success=1
  fi
  finished_at="$(date +%s)"
  duration=$((finished_at - started_at))

  printf 'illarin.backup.success:%d|g|#env:production\n' "$success" >/dev/udp/127.0.0.1/8125 || true
  printf 'illarin.backup.duration:%d|g|#env:production\n' "$duration" >/dev/udp/127.0.0.1/8125 || true
  exit "$status"
}

if [[ "$action" == "run" ]]; then
  trap report_result EXIT
fi

compose --profile tools run --rm backup "$action"
