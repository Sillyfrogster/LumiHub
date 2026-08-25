#!/usr/bin/env bash

set -euo pipefail

OPS_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$OPS_DIR/.." && pwd)"
TEST_DIR="$(mktemp -d /tmp/illarin-stack.XXXXXX)"
TEST_ENV="$TEST_DIR/production.env"
MICROSOFT_TEST_ENV="$TEST_DIR/microsoft-production.env"
TEST_PROJECT="illarin-smoke"
TEST_PORT="${ILLARIN_TEST_PORT:-18080}"
VERSION="$(git -C "$REPO_ROOT" rev-parse HEAD)"

compose_test() {
  docker compose \
    --env-file "$TEST_ENV" \
    -p "$TEST_PROJECT" \
    -f "$REPO_ROOT/compose.prod.yaml" \
    "$@"
}

compose_microsoft_test() {
  docker compose \
    --env-file "$MICROSOFT_TEST_ENV" \
    -p "$TEST_PROJECT-microsoft" \
    -f "$REPO_ROOT/compose.prod.yaml" \
    -f "$REPO_ROOT/compose.microsoft365.yaml" \
    "$@"
}

cleanup() {
  local status="$?"
  trap - EXIT

  if (( status != 0 )); then
    compose_test logs --no-color --tail=200 || true
  fi
  compose_test down --volumes --remove-orphans >/dev/null 2>&1 || true
  case "$TEST_DIR" in
    /tmp/illarin-stack.*)
      docker run --rm -v "$TEST_DIR:/test-data" alpine:3.23 \
        chown -R "$(id -u):$(id -g)" /test-data >/dev/null 2>&1 || true
      rm -rf -- "$TEST_DIR"
      ;;
  esac
  exit "$status"
}
trap cleanup EXIT

for command in docker curl; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "$command is required for the production stack test." >&2
    exit 1
  fi
done

mkdir -p \
  "$TEST_DIR/data/postgres" \
  "$TEST_DIR/data/uploads/blobs" \
  "$TEST_DIR/data/uploads/derivatives" \
  "$TEST_DIR/data/backup-work" \
  "$TEST_DIR/secrets"
chmod 0777 "$TEST_DIR/data/uploads" "$TEST_DIR/data/uploads/blobs" "$TEST_DIR/data/uploads/derivatives"
printf '%s' 'test-client-secret' >"$TEST_DIR/secrets/microsoft-365-client-secret"

{
  printf 'ILLARIN_IMAGE_REGISTRY=local\n'
  printf 'ILLARIN_VERSION=%s\n' "$VERSION"
  printf 'ILLARIN_DATA_DIR=%s/data\n' "$TEST_DIR"
  printf 'ILLARIN_SECRETS_DIR=%s/secrets\n' "$TEST_DIR"
  printf 'ILLARIN_GATEWAY_BIND=127.0.0.1\n'
  printf 'ILLARIN_GATEWAY_PORT=%s\n' "$TEST_PORT"
  printf 'NPMPLUS_NETWORK=\n'
  printf 'SITE_URL=http://127.0.0.1:%s\n' "$TEST_PORT"
  printf 'POSTGRES_DB=illarin\n'
  printf 'POSTGRES_USER=illarin\n'
  printf 'POSTGRES_PASSWORD=illarin-test-password\n'
  printf 'DATABASE_URL=postgres://illarin:illarin-test-password@db:5432/illarin\n'
  printf 'LINKING_HMAC_KEY=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n'
  printf 'SMTP_ADDR=smtp.illarin.test:25\n'
  printf 'SMTP_FROM=mail@illarin.test\n'
  printf 'DD_API_KEY=00000000000000000000000000000000\n'
  printf 'BACKUPS_ENABLED=false\n'
} >"$TEST_ENV"
grep -v '^SMTP_' "$TEST_ENV" >"$MICROSOFT_TEST_ENV"
{
  printf 'MICROSOFT_365_TENANT_ID=test-tenant\n'
  printf 'MICROSOFT_365_CLIENT_ID=test-client\n'
  printf 'MICROSOFT_365_MAILBOX=mail@illarin.test\n'
} >>"$MICROSOFT_TEST_ENV"
chmod 0600 "$TEST_ENV"
chmod 0600 "$MICROSOFT_TEST_ENV"
set -a
# shellcheck disable=SC1090
source "$TEST_ENV"
set +a

docker tag illarin-api:local "local/illarin-api:$VERSION"
docker tag illarin-web:local "local/illarin-web:$VERSION"

compose_test config --quiet
if ! compose_test config | grep -Fq 'SMTP_ADDR: smtp.illarin.test:25'; then
  echo "The production stack did not pass SMTP settings to the API." >&2
  exit 1
fi
compose_microsoft_test config --quiet
compose_test up -d --wait --wait-timeout 180 db
compose_test --profile tools run --rm migrate
compose_test up -d --wait --wait-timeout 240 api web gateway
compose_test ps

curl --fail --silent --show-error "http://127.0.0.1:$TEST_PORT/gateway-healthz" --output /dev/null
curl --fail --silent --show-error "http://127.0.0.1:$TEST_PORT/api/readyz" --output /dev/null
curl --fail --silent --show-error "http://127.0.0.1:$TEST_PORT/" --output /dev/null
curl --fail --silent --show-error "http://127.0.0.1:$TEST_PORT/openapi.yaml" --output /dev/null

internal_status="$(curl --silent --output /dev/null --write-out '%{http_code}' "http://127.0.0.1:$TEST_PORT/_illarin/blobs/not-public")"
if [[ "$internal_status" != "404" ]]; then
  echo "The internal blob location returned $internal_status instead of 404." >&2
  exit 1
fi

echo "The isolated production stack passed."
