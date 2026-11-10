#!/usr/bin/env bash
# @@License : WTFPL

set -Eeuo pipefail
# shellcheck source=tests/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_command make

printf 'Building server and CLI binaries in Docker...\n'
mkdir -p "$TEST_DIR/anime"
tar -C "$PROJECT_DIR" --exclude=.git --exclude=binaries --exclude=releases --exclude=tests -cf - . | tar -C "$TEST_DIR/anime" -xf -
mkdir -p "$TEST_DIR/go-cache" "$TEST_DIR/go-build"
make -C "$TEST_DIR/anime" build \
    GO_CACHE="$TEST_DIR/go-cache" GO_BUILD="$TEST_DIR/go-build" \
    BINDIR=binaries
cp "$TEST_DIR/anime/binaries/anime-linux-amd64" "$TEST_DIR/bin/anime"
cp "$TEST_DIR/anime/binaries/anime-cli-linux-amd64" "$TEST_DIR/bin/anime-cli"

require_command jq
docker run --rm --name "${CONTAINER_NAME}-binary" \
    -v "$TEST_DIR/bin:/app:ro" alpine:latest sh -c \
    'apk add --no-cache bash curl file jq >/dev/null && /app/anime --version && /app/anime --help >/dev/null && /app/anime-cli --help >/dev/null && file /app/anime /app/anime-cli'
start_test_server
run_api_smoke_tests
docker run --rm --network host --name "${CONTAINER_NAME}-cli" \
    -v "$TEST_DIR/bin:/app:ro" -e BASE_URL="$BASE_URL" alpine:latest sh -c \
    'apk add --no-cache curl jq >/dev/null && /app/anime-cli --server "$BASE_URL" random --output json | jq -e ".anime and .quote and .character" >/dev/null'
printf 'PASS: Docker binary validation\n'
