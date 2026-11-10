#!/usr/bin/env bash
# @@License : WTFPL

set -Eeuo pipefail
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
if command -v incus >/dev/null 2>&1 && incus info >/dev/null 2>&1; then
    exec "$SCRIPT_DIR/incus.sh" "$@"
fi
exec "$SCRIPT_DIR/docker.sh" "$@"
