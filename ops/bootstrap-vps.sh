#!/usr/bin/env bash

set -euo pipefail

if (( EUID != 0 )); then
  echo "Run this bootstrap with sudo." >&2
  exit 1
fi

if (( $# != 4 )); then
  echo "Usage: bootstrap-vps.sh DEPLOY_USER ENV_FILE MICROSOFT_SECRET DEPLOY_PUBLIC_KEY" >&2
  exit 1
fi

deploy_user="$1"
source_env="$2"
microsoft_secret="$3"
deploy_public_key="$4"

for file in "$source_env" "$microsoft_secret" "$deploy_public_key"; do
  if [[ ! -f "$file" ]]; then
    echo "Required setup file is missing: $file" >&2
    exit 1
  fi
done

if ! id "$deploy_user" >/dev/null 2>&1; then
  echo "The deployment user does not exist: $deploy_user" >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
  echo "Install Docker Engine with the Compose plugin before running this bootstrap." >&2
  exit 1
fi
if ! command -v flock >/dev/null 2>&1; then
  echo "Install util-linux before running this bootstrap." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$source_env"
set +a

ILLARIN_DATA_DIR="${ILLARIN_DATA_DIR:-/srv/illarin}"
ILLARIN_SECRETS_DIR="${ILLARIN_SECRETS_DIR:-/etc/illarin/secrets}"
case "$ILLARIN_DATA_DIR" in
  /srv/illarin|/var/lib/illarin) ;;
  *)
    echo "ILLARIN_DATA_DIR must be /srv/illarin or /var/lib/illarin." >&2
    exit 1
    ;;
esac
case "$ILLARIN_SECRETS_DIR" in
  /etc/illarin/secrets) ;;
  *)
    echo "ILLARIN_SECRETS_DIR must be /etc/illarin/secrets." >&2
    exit 1
    ;;
esac

deploy_group="$(id -gn "$deploy_user")"
deploy_home="$(getent passwd "$deploy_user" | cut -d: -f6)"
if [[ -z "$deploy_home" || ! -d "$deploy_home" ]]; then
  echo "The deployment user's home directory is unavailable." >&2
  exit 1
fi

install -d -m 0755 -o "$deploy_user" -g "$deploy_group" /opt/illarin /opt/illarin/releases
install -d -m 0750 -o "$deploy_user" -g "$deploy_group" /etc/illarin "$ILLARIN_SECRETS_DIR"
install -d -m 0755 "$ILLARIN_DATA_DIR" "$ILLARIN_DATA_DIR/postgres"
install -d -m 0755 -o 10001 -g 10001 \
  "$ILLARIN_DATA_DIR/uploads" \
  "$ILLARIN_DATA_DIR/uploads/blobs" \
  "$ILLARIN_DATA_DIR/uploads/derivatives"
install -d -m 0700 -o 10001 -g 10001 "$ILLARIN_DATA_DIR/backup-work"
install -d -m 0750 -o "$deploy_user" -g "$deploy_group" \
  "$ILLARIN_DATA_DIR/releases" \
  "$ILLARIN_DATA_DIR/locks"

install -m 0600 -o "$deploy_user" -g "$deploy_group" "$source_env" /etc/illarin/production.env
install -m 0600 -o "$deploy_user" -g "$deploy_group" \
  "$microsoft_secret" "$ILLARIN_SECRETS_DIR/microsoft-365-client-secret"

public_key="$(<"$deploy_public_key")"
if [[ ! "$public_key" =~ ^ssh-ed25519[[:space:]] ]]; then
  echo "The deployment public key is not an Ed25519 SSH key." >&2
  exit 1
fi
install -d -m 0700 -o "$deploy_user" -g "$deploy_group" "$deploy_home/.ssh"
touch "$deploy_home/.ssh/authorized_keys"
chown "$deploy_user:$deploy_group" "$deploy_home/.ssh/authorized_keys"
chmod 0600 "$deploy_home/.ssh/authorized_keys"
if ! grep -Fqx "$public_key" "$deploy_home/.ssh/authorized_keys"; then
  printf '%s\n' "$public_key" >>"$deploy_home/.ssh/authorized_keys"
fi

if getent group docker >/dev/null 2>&1; then
  usermod -aG docker "$deploy_user"
fi

upsert_env() {
  local key="$1"
  local value="$2"
  local destination=/etc/illarin/production.env
  local temporary

  temporary="$(mktemp /etc/illarin/.production.env.XXXXXX)"
  grep -vE "^${key}=" "$destination" >"$temporary" || true
  printf '%s=%s\n' "$key" "$value" >>"$temporary"
  chown "$deploy_user:$deploy_group" "$temporary"
  chmod 0600 "$temporary"
  mv "$temporary" "$destination"
}

npmplus_container="${NPMPLUS_CONTAINER:-}"
if [[ -z "$npmplus_container" ]]; then
  npmplus_container="$(docker ps --format '{{.Names}}' | grep -i -m1 'npmplus\|nginx-proxy-manager' || true)"
fi

npmplus_target=""
if [[ -n "$npmplus_container" ]]; then
  network_mode="$(docker inspect --format '{{.HostConfig.NetworkMode}}' "$npmplus_container")"
  if [[ "$network_mode" == "host" ]]; then
    upsert_env NPMPLUS_NETWORK ""
    upsert_env ILLARIN_GATEWAY_BIND 127.0.0.1
    npmplus_target="127.0.0.1:8000"
  else
    npmplus_networks="$(docker inspect --format '{{range $name, $_ := .NetworkSettings.Networks}}{{$name}} {{end}}' "$npmplus_container")"
    npmplus_network="${npmplus_networks%% *}"
    if [[ -n "$npmplus_network" && "$npmplus_network" != "bridge" ]]; then
      upsert_env NPMPLUS_NETWORK "$npmplus_network"
      upsert_env ILLARIN_GATEWAY_BIND 127.0.0.1
      npmplus_target="illarin-gateway:8080"
    else
      bridge_gateway="$(docker network inspect bridge --format '{{(index .IPAM.Config 0).Gateway}}')"
      upsert_env NPMPLUS_NETWORK ""
      upsert_env ILLARIN_GATEWAY_BIND "$bridge_gateway"
      npmplus_target="$bridge_gateway:8000"
    fi
  fi
fi

if [[ -n "$npmplus_target" ]]; then
  printf '%s\n' "$npmplus_target" >/etc/illarin/npmplus-target
  chown "$deploy_user:$deploy_group" /etc/illarin/npmplus-target
  chmod 0640 /etc/illarin/npmplus-target
  echo "NPMPlus should forward HTTP traffic to $npmplus_target."
else
  echo "NPMPlus was not detected. Configure an edge proxy before exposing the gateway." >&2
fi

echo "The host directories, production settings, Microsoft secret, and deployment key are ready."
echo "Sign out and back in before deploying so Docker group membership takes effect."
