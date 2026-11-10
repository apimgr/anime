#!/usr/bin/env bash
# @@License : WTFPL

set -Eeuo pipefail

PROJECT_DIR=$(git -C "$(dirname "${BASH_SOURCE[0]}")/.." rev-parse --show-toplevel)
PROJECT_NAME=$(basename "$PROJECT_DIR")
PROJECT_ORG=$(git -C "$PROJECT_DIR" remote get-url origin 2>/dev/null | sed -E 's|.*/([^/]+)/[^/]+(\.git)?$|\1|' || true)
PROJECT_ORG=${PROJECT_ORG:-$(basename "$(dirname "$PROJECT_DIR")")}
TEMP_BASE=${TMPDIR:-/tmp}
mkdir -p "$TEMP_BASE/$PROJECT_ORG"
TEST_DIR=$(mktemp -d "$TEMP_BASE/$PROJECT_ORG/$PROJECT_NAME-XXXXXX")
CONTAINER_NAME="${PROJECT_NAME}-test-$(printf '%04x%04x' "$RANDOM" "$RANDOM")"
SERVER_CONTAINER=

cleanup_test_environment() {
    if [[ -n "$SERVER_CONTAINER" ]]; then
        docker rm -f "$SERVER_CONTAINER" >/dev/null 2>&1 || true
    fi
    for container in ${EXTRA_CONTAINERS:-}; do
        docker rm -f "$container" >/dev/null 2>&1 || true
    done
    if [[ -n "${INCUS_INSTANCE:-}" ]]; then
        incus delete --force "$INCUS_INSTANCE" >/dev/null 2>&1 || true
    fi
    rm -rf "$TEST_DIR"
}
trap cleanup_test_environment EXIT INT TERM

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "required command is missing: $1"
}

build_test_binaries() {
    require_command docker
    mkdir -p "$TEST_DIR/bin" "$TEST_DIR/go-cache" "$TEST_DIR/go-build"
    docker run --rm --name "${CONTAINER_NAME}-build" \
        --memory=4g --cpus=2 \
        -v "$PROJECT_DIR:/app" -v "$TEST_DIR/bin:/out" \
        -v "$TEST_DIR/go-cache:/usr/local/share/go/pkg/mod" \
        -v "$TEST_DIR/go-build:/usr/local/share/go/cache" \
        -w /app -e CGO_ENABLED=0 -e GOFLAGS=-buildvcs=false \
        casjaysdev/go:latest sh -c \
        'go build -buildvcs=false -trimpath -o /out/anime ./src && go build -buildvcs=false -trimpath -o /out/anime-cli ./src/client'
}

start_test_server() {
    local port=${1:-$((49000 + RANDOM % 10000))}
    require_command docker
    mkdir -p "$TEST_DIR/config" "$TEST_DIR/data" "$TEST_DIR/logs" "$TEST_DIR/cache" "$TEST_DIR/backup"
    SERVER_CONTAINER=$CONTAINER_NAME
    docker run --rm -d --name "$SERVER_CONTAINER" \
        --publish "127.0.0.1:$port:8080" \
        --volume "$TEST_DIR:/state" \
        alpine:latest /bin/sh -c \
        'apk add --no-cache ca-certificates >/dev/null && exec /state/bin/anime --address 0.0.0.0 --port 8080 --config /state/config --data /state/data --cache /state/cache --log /state/logs --backup /state/backup' >/dev/null
    BASE_URL="http://127.0.0.1:$port"
    export BASE_URL
    for _ in {1..45}; do
        if curl -q -fsS "$BASE_URL/api/v1/server/healthz" >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    docker logs "$SERVER_CONTAINER" >&2 || true
    fail 'server did not become healthy'
}

assert_status() {
    local expected=$1 path=$2 actual
    actual=$(curl -q -sS -o "$TEST_DIR/response" -w '%{http_code}' "$BASE_URL$path") || true
    [[ "$actual" == "$expected" ]] || fail "$path returned HTTP $actual, expected $expected"
}

run_api_smoke_tests() {
    require_command curl
    local response
    response=$(curl -q -fsS -H 'Accept: application/json' "$BASE_URL/api/v1/random") || fail 'random quote JSON endpoint failed'
    printf '%s' "$response" | jq -e '.data.anime and .data.quote and .data.character' >/dev/null || fail 'random quote response is missing required fields'
    curl -q -fsS -H 'Accept: text/plain' "$BASE_URL/api/v1/random" | grep -q '[^[:space:]]' || fail 'random quote text negotiation returned empty content'
    curl -q -fsS "$BASE_URL/api/v1/random.txt" | grep -q '[^[:space:]]' || fail 'random quote .txt endpoint returned empty content'
    curl -q -fsS -H 'Accept: application/json' "$BASE_URL/api/v1/quotes" | jq -e '.data | type == "array" and length > 0' >/dev/null || fail 'quote collection JSON endpoint failed'
    curl -q -fsS -H 'Accept: text/plain' "$BASE_URL/api/v1/quotes" | grep -q '[^[:space:]]' || fail 'quote collection text negotiation returned empty content'
    curl -q -fsS "$BASE_URL/api/v1/quotes.txt" | grep -q '[^[:space:]]' || fail 'quote collection .txt endpoint returned empty content'
    curl -q -fsS "$BASE_URL/robots.txt" | grep -q 'User-agent' || fail 'robots.txt is invalid'
    curl -q -fsS "$BASE_URL/.well-known/security.txt" | grep -q '[^[:space:]]' || fail 'security.txt is empty'
    curl -q -fsS "$BASE_URL/api/v1/server/healthz" | jq -e '.data.status == "healthy"' >/dev/null || fail 'health JSON endpoint failed'
    curl -q -fsS "$BASE_URL/api/v1/server/healthz.txt" | grep -q 'Status: healthy' || fail 'health text endpoint failed'
    curl -q -fsS "$BASE_URL/api/v1/stats" | jq -e . >/dev/null || fail 'stats JSON endpoint failed'
    curl -q -fsS "$BASE_URL/api/v1/stats.txt" | grep -q '[^[:space:]]' || fail 'stats text endpoint failed'
    local html plain
    html=$(curl -q -fsS -H 'Accept: text/html' "$BASE_URL/") || fail 'home HTML request failed'
    [[ "$html" == *'<!DOCTYPE html'* ]] || fail 'home page did not return server-rendered HTML'
    plain=$(curl -q -fsS -H 'Accept: text/plain' "$BASE_URL/") || fail 'home text request failed'
    [[ "$plain" != *'<html'* ]] || fail 'home text negotiation returned HTML'
    printf 'PASS: API, text, health, and frontend smoke tests\n'
}
