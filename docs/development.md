# Development

Anime Quotes is a Go application. Read [AI.md](https://github.com/apimgr/anime/blob/main/AI.md) for implementation rules and [IDEA.md](https://github.com/apimgr/anime/blob/main/IDEA.md) for product requirements before changing code. Go builds and tests run in the repository's `casjaysdev/go:latest` Docker toolchain through the Makefile; do not install or run Go directly on the host.

## Prerequisites

- Git
- GNU Make
- Docker with permission to run containers, including for the documentation build
- Incus only for the optional full application test tier

## Local workflow

Clone the repository and enter its directory:

```sh
git clone https://github.com/apimgr/anime.git
cd anime
```

Use the Makefile for Go tasks:

```sh
make dev       # quick server and CLI build in an OS temporary directory
make test      # go vet, govulncheck, unit tests and the 60% coverage gate
make local     # production-style server and CLI binaries for the host platform
make build     # local binaries plus all supported OS and architecture builds
make clean     # remove binaries/ and releases/
```

Maintainers publish a beta by running `make beta` on a clean `main` branch. It creates and pushes a UTC `YYYYMMDDHHMMSS-beta` tag; the CI workflows build and publish the beta release and images.

Runtime and test data belongs in an OS temporary directory. Do not use the production or development Compose files for automated testing. CI uses explicit Go commands in the maintained toolchain container and does not call Makefile targets. CI definitions are maintained for GitHub Actions, Gitea Actions, Forgejo Actions, GitLab CI, and Jenkins; the GitHub workflows run for this repository's GitHub remote.

## Integration and browser testing

The basic binary and HTTP checks are Docker-only:

```sh
./tests/run_tests.sh
./tests/docker.sh
./tests/test_content_negotiation.sh
```

`run_tests.sh` delegates only to the Docker basic tier; it never falls back to Incus. If Docker is unavailable, it reports the missing prerequisite and exits with status 69. Other test scripts use the shared prerequisite check, which also reports a missing command and exits with status 69. The Docker tier checks both server and CLI version/help output, then copies each binary under a renamed filename and verifies the renamed name appears in its help output. The content-negotiation script separately checks the public HTML, API (including `/api/autodiscover`), and fixed discovery routes with their supported response formats.

The Incus service lifecycle checks are explicit manual runs, separate from the Docker basic tier:

```sh
./tests/incus.sh systemd
./tests/incus.sh openrc
./tests/incus.sh all
```

The on-demand headless browser suite covers server-rendered, no-JavaScript, and JavaScript-enabled flows. Its responsive layout matrix checks the public page routes in English and Arabic at the nine AI.md viewport sizes, in dark, light, and automatic themes, with 200% text zoom, long identifiers and translations, empty/small/full lists, and labeled/touch-sized controls. The horizontal-overflow backstop is disabled during those checks, and the mobile drawer is checked for viewport height and complete navigation. It is a separate manual tier and is not started by `run_tests.sh`, `make test`, or the commit gate:

```sh
./tests/e2e.sh
```

The same test tiers can run after the unit, vet, and vulnerability gate through
Make variables: `make test BASIC=1 CONTENT_NEGOTIATION=1 INCUS=1 E2E=1`.

Use only the test environments described by AI.md. Do not start `docker/docker-compose.yml` or `docker/docker-compose.dev.yml` during automated testing.

## Documentation

Build the Read the Docs site in a disposable Docker container. This keeps Python packages and generated site files out of the host and repository:

```sh
PROJECT_REMOTE=$(git remote get-url origin)
PROJECT_NAME=${PROJECT_REMOTE##*/}
PROJECT_NAME=${PROJECT_NAME%.git}
PROJECT_ORG=$(printf '%s\n' "$PROJECT_REMOTE" | sed -E 's|.*/([^/]+)/[^/]+(\.git)?$|\1|')
INTERNAL_NAME=$(sed -n 's/^internal_name:[[:space:]]*//p' IDEA.md | head -n1)
INTERNAL_NAME=${INTERNAL_NAME:-$PROJECT_NAME}
mkdir -p "${TMPDIR:-/tmp}/$PROJECT_ORG"
DOCS_TMP=$(mktemp -d "${TMPDIR:-/tmp}/$PROJECT_ORG/$INTERNAL_NAME-XXXXXXXX")
mkdir -p "$DOCS_TMP/site"
DOCS_NAME="$PROJECT_NAME-$(od -An -N4 -tx1 /dev/urandom | tr -d ' ')"
trap 'rm -rf "$DOCS_TMP"' EXIT
timeout -k 30 "${DOCKER_TIMEOUT:-3600}" docker run --rm --name "$DOCS_NAME" --no-healthcheck --memory=2g --cpus=2 \
  -v "$PWD:/src:ro" -v "$DOCS_TMP/site:/site" -w /src python:3.12-alpine \
  sh -ec 'python -m pip install --no-cache-dir -r docs/requirements.txt && mkdocs build --strict --site-dir /site'
```

Update the relevant docs when changing routes, configuration, user workflows, CLI behavior, translations, or deployment. Check locale catalogs for matching keys when adding translated text.

## Code and pull requests

Keep changes focused, add or update tests for behavior changes, and update documentation in the same change. Use descriptive branches and pull requests against the default branch. Include the reason for the change, the checks run, any configuration or documentation updates, and security or privacy impact in the pull request description. Do not include credentials or private data in code, tests, documentation, or logs.

Report suspected vulnerabilities through [GitHub private vulnerability reporting](https://github.com/apimgr/anime/security/advisories/new), not a public issue. See [Security](security.md) for the application's reporting routes and response process.
