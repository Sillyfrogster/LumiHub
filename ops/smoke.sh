#!/usr/bin/env bash

set -euo pipefail

OPS_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$OPS_DIR/lib.sh"

require_release

compose exec -T gateway wget -q -T 5 -O /dev/null http://127.0.0.1:8080/gateway-healthz
compose exec -T gateway wget -q -T 10 -O /dev/null http://127.0.0.1:8080/api/readyz
compose exec -T gateway wget -q -T 15 -O /dev/null http://127.0.0.1:8080/

echo "Illarin passed its gateway, API, and site smoke checks."
