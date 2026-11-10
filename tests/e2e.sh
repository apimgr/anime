#!/usr/bin/env bash
# @@License : WTFPL

set -Eeuo pipefail
# shellcheck source=tests/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_command docker

build_test_binaries
docker run --rm --name "${CONTAINER_NAME}-binary" \
    -v "$TEST_DIR/bin:/app:ro" alpine:latest /app/anime --version
start_test_server

CHROME_CONTAINER="${CONTAINER_NAME}-chrome"
docker run --rm -d --name "$CHROME_CONTAINER" --network host \
    chromedp/headless-shell:latest \
    --no-sandbox --remote-debugging-address=127.0.0.1 --remote-debugging-port=9222 >/dev/null
export EXTRA_CONTAINERS=$CHROME_CONTAINER
for _ in {1..30}; do
    if curl -q -fsS http://127.0.0.1:9222/json/version >/dev/null 2>&1; then
        break
    fi
    sleep 1
done
curl -q -fsS http://127.0.0.1:9222/json/version >/dev/null || fail 'headless Chromium did not become ready'

docker run --rm --name "${CONTAINER_NAME}-e2e" --network host \
    -v "$PROJECT_DIR:/app:ro" \
    -v "$TEST_DIR/go-cache:/usr/local/share/go/pkg/mod" \
    -v "$TEST_DIR/go-build:/usr/local/share/go/cache" \
    -w /app -e GOFLAGS=-buildvcs=false \
    -e E2E_BASE_URL="$BASE_URL" -e E2E_CHROME_URL=ws://127.0.0.1:9222 \
    casjaysdev/go:latest go test -tags e2e -count=1 ./tests/e2e/...
