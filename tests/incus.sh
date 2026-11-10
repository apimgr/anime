#!/usr/bin/env bash
# @@License : WTFPL

set -Eeuo pipefail
# shellcheck source=tests/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_command incus
require_command make
INCUS_INSTANCE="${PROJECT_NAME}-test-$(printf '%04x%04x' "$RANDOM" "$RANDOM")"

mkdir -p "$TEST_DIR/anime"
tar -C "$PROJECT_DIR" --exclude=.git --exclude=binaries --exclude=releases --exclude=tests -cf - . | tar -C "$TEST_DIR/anime" -xf -
mkdir -p "$TEST_DIR/go-cache" "$TEST_DIR/go-build"
make -C "$TEST_DIR/anime" build \
    GO_CACHE="$TEST_DIR/go-cache" GO_BUILD="$TEST_DIR/go-build" BINDIR=binaries
incus launch images:debian/12 "$INCUS_INSTANCE"
incus exec "$INCUS_INSTANCE" -- mkdir -p /opt/anime /var/lib/anime/config /var/lib/anime/data /var/lib/anime/cache /var/log/anime /var/lib/anime/backup
incus file push "$TEST_DIR/anime/binaries/anime-linux-amd64" "$INCUS_INSTANCE/opt/anime/anime"
incus exec "$INCUS_INSTANCE" -- chmod 0755 /opt/anime/anime
incus exec "$INCUS_INSTANCE" -- /opt/anime/anime --version
incus exec "$INCUS_INSTANCE" -- /opt/anime/anime --help
printf 'PASS: Incus Debian binary validation. Full service lifecycle checks require the service installer.\n'
