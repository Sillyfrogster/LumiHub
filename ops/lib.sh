#!/usr/bin/env bash

set -euo pipefail

OPS_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$OPS_DIR/.." && pwd)"
ILLARIN_ENV_FILE="${ILLARIN_ENV_FILE:-/etc/illarin/production.env}"

if [[ ! -f "$ILLARIN_ENV_FILE" ]]; then
  echo "Production settings are missing: $ILLARIN_ENV_FILE" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ILLARIN_ENV_FILE"
set +a

ILLARIN_DATA_DIR="${ILLARIN_DATA_DIR:-/srv/illarin}"
export ILLARIN_DATA_DIR ILLARIN_ENV_FILE

if [[ -z "${ILLARIN_VERSION:-}" && -f "$ILLARIN_DATA_DIR/releases/current" ]]; then
  ILLARIN_VERSION="$(<"$ILLARIN_DATA_DIR/releases/current")"
  export ILLARIN_VERSION
fi

compose() {
  local files=(
    --env-file "$ILLARIN_ENV_FILE"
    -f "$REPO_ROOT/compose.prod.yaml"
  )
  if [[ -n "${MICROSOFT_365_TENANT_ID:-}${MICROSOFT_365_CLIENT_ID:-}${MICROSOFT_365_MAILBOX:-}" ]]; then
    files+=(-f "$REPO_ROOT/compose.microsoft365.yaml")
  fi
  if [[ -n "${NPMPLUS_NETWORK:-}" ]]; then
    files+=(-f "$REPO_ROOT/compose.npmplus.yaml")
  fi
  docker compose "${files[@]}" "$@"
}

require_release() {
  if [[ ! "${ILLARIN_VERSION:-}" =~ ^[0-9a-f]{40}$ ]]; then
    echo "ILLARIN_VERSION must be a full lowercase Git commit SHA." >&2
    exit 1
  fi
}

take_operation_lock() {
  mkdir -p "$ILLARIN_DATA_DIR/locks"
  exec 9>"$ILLARIN_DATA_DIR/locks/operations.lock"
  if ! flock -n 9; then
    echo "Another Illarin deploy or backup is already running." >&2
    exit 1
  fi
}

write_release() {
  local name="$1"
  local version="$2"
  local directory="$ILLARIN_DATA_DIR/releases"
  local temporary

  mkdir -p "$directory"
  temporary="$(mktemp "$directory/.${name}.XXXXXX")"
  printf '%s\n' "$version" >"$temporary"
  mv "$temporary" "$directory/$name"
}
