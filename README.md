# Anime Quotes

[![CI](https://github.com/apimgr/anime/actions/workflows/ci.yml/badge.svg)](https://github.com/apimgr/anime/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/apimgr/anime)](https://github.com/apimgr/anime/releases)
[![License](https://img.shields.io/github/license/apimgr/anime)](LICENSE.md)
[![Docs](https://readthedocs.org/projects/apimgr-anime/badge/?version=latest)](https://apimgr-anime.readthedocs.io/)

## About

Anime Quotes is a full-stack Go application serving anime quotes from a curated, embedded collection. Visitors can browse the server-rendered site, use the versioned REST API or read-only GraphQL API, and access the service with the companion CLI. Public quote reads require no account or token. MCP and token-management operations use anonymous owner tokens; the application has no human accounts.

## Features

- Random quote retrieval, full collection browsing, and search by quote text, anime, or character
- REST API with OpenAPI documentation and a read-only GraphQL API with browser explorer
- Companion `anime-cli` for random, list, and search operations
- Server-rendered, responsive web interface, preferences, consent controls, and PWA support
- Anonymous owner-token management for MCP access and separate linked tokens per client
- Contact and security reporting with CAPTCHA, abuse controls, encrypted report storage, and OpenPGP support
- Public discovery through `robots.txt`, `sitemap.xml`, Atom/RSS feeds, `llms.txt`, and canonical `security.txt`
- Application-managed Tor and I2P overlay support
- Persistent scheduled maintenance, encrypted and verified backups, SMTP operator alerts, GeoIP/blocklist/CVE data, and authenticated Prometheus metrics
- Service installation across systemd, OpenRC, SysVinit, runit, s6, launchd, BSD rc.d, and Windows Service Control Manager, with a dedicated Unix service account
- Verified server self-updates with defer windows and staged restart handling

## Production

### Docker Compose

Review `docker/docker-compose.yml`, especially its published port and volume paths, then start the service:

```sh
docker compose -f docker/docker-compose.yml up -d
```

The service listens on port 80 in the container. Configuration, data, and logs are mounted under `/config/anime`, `/data/anime`, and `/data/log/anime`. The Compose file binds host port `64580` on the Docker bridge address. The application starts Tor automatically when its binary is available; its onion address is shown on `/server/help` after publication. I2P is enabled by default in container mode and can be configured under `server.i2p`.

### Binary and service installation

Download the server artifact for your platform from [GitHub Releases](https://github.com/apimgr/anime/releases/latest). The server creates its configuration and operator token on first run, then selects an available port when no port is configured. Run `anime --help` for options. To install the detected system service, run `anime --service --install`; use the platform's service manager to inspect and control it, or `anime --status` for the application health check. Installation and platform details are in [Installation](docs/installation.md).

| Platform | Architectures | Server artifact |
|---|---|---|
| Linux | amd64, arm64 | `anime-linux-{arch}` |
| macOS | amd64, arm64 | `anime-darwin-{arch}` |
| Windows | amd64, arm64 | `anime-windows-{arch}.exe` |
| FreeBSD | amd64, arm64 | `anime-freebsd-{arch}` |

## Client

Download the matching `anime-cli` artifact from [GitHub Releases](https://github.com/apimgr/anime/releases). This repository does not define an official hosted service, so provide the server URL explicitly:

```sh
anime-cli --server https://YOUR_SERVER random
anime-cli --server https://YOUR_SERVER --output json list
anime-cli --server https://YOUR_SERVER search "fullmetal alchemist"
```

`--server` overrides `ANIME_SERVER_PRIMARY`, which overrides the selected CLI profile. The client uses public read-only quote routes.

## Configuration

The server reads `server.yml`, ordered area files under `yaml.d/`, and `secrets.yml` in its configuration directory. First run generates the operator token in `secrets.yml`; keep that file private. Directory overrides are `CONFIG_DIR`, `DATA_DIR`, `CACHE_DIR`, `LOG_DIR`, `BACKUP_DIR`, `PID_FILE`, and `DATABASE_DIR`, with matching CLI flags where supported. `PORT` and `LISTEN` provide listen defaults, and `--port` and `--address` override them.

See [Configuration](docs/configuration.md) for area-file placement, defaults, migrations, secrets, and the CLI profile format.

## API

Public quote reads are anonymous. MCP and token-management routes require an owner Bearer token, except primary-token issuance. All API routes are under `/api/v1` unless noted.

| Method | Route | Description |
|---|---|---|
| GET | `/api/v1` and `/api` | API route index |
| GET | `/api/v1/random` | Return one random quote |
| GET | `/api/v1/quotes` | List or search quotes with optional `q`, `anime`, and `character` filters |
| GET | `/api/v1/stats` | Runtime and collection statistics |
| GET | `/api/v1/server/{about,help,privacy,terms,dpo}` | Informational content and privacy contact |
| GET | `/api/v1/server/contact` | Contact/reporting page data |
| GET | `/api/v1/server/security` | Security reporting metadata and disclosure links |
| GET | `/api/v1/server/security/{policy,thanks}` | Disclosure policy and acknowledgments |
| GET | `/api/v1/server/healthz` | Health object; HTTP 503 when unhealthy |
| GET, POST | `/api/v1/server/preferences` | Read or update browser-local preferences |
| GET | `/api/v1/server/preferences/sync` | Export preferences as a share URL and code |
| GET | `/api/v1/server/preferences/sync/import` | Apply supported preferences from a share URL, query, or code |
| GET | `/api/v1/server/preferences/tokens` | Token list; values are never returned by list operations |
| POST | `/api/v1/server/preferences/tokens/primary` | Issue a primary owner token |
| DELETE | `/api/v1/server/preferences/tokens/primary` | Revoke an owner after confirming the token's final four characters |
| POST, PATCH, DELETE | `/api/v1/server/preferences/tokens/...` | Reveal, rotate, create, update, or revoke tokens; primary owner required |
| POST | `/api/v1/server/mcp` | MCP Streamable HTTP; Bearer owner token required |
| GET | `/api/v1/server/help/mcp` | MCP connection guide and tool list |
| GET | `/api/v1/server/swagger` | OpenAPI 3.1 JSON |
| GET, POST | `/api/v1/server/graphql` | Read-only GraphQL query endpoint |
| GET | `/api/v1/{random,quotes,stats}.txt` | Plain-text responses |
| GET | `/api/autodiscover` | CLI release metadata |
| GET | `/cli/binaries/{file}` | Supported CLI release artifact |

Direct aliases include `/api/healthz`, `/api/swagger`, `/api/graphql`, `/api/mcp`, `/mcp`, and `/graphql`. The browser pages include `/`, `/quotes`, the `/server/*` information and preference pages, `/server/contact`, `/server/security`, `/server/help/mcp`, `/server/docs/swagger`, and `/server/docs/graphql`. Token action routes and preference-sync import are POST/GET actions and are not pages. See the [API reference](docs/api.md) for response formats and authentication details.

## Other

- **Languages:** Embedded locale catalogs cover English (`en`), Spanish (`es`), French (`fr`), German (`de`), Italian (`it`), and Arabic (`ar`). The server detects browser language preferences and lets visitors choose a language on `/server/preferences`; `anime-cli --lang CODE` selects the request language.
- [Installation](docs/installation.md)
- [CLI reference](docs/cli.md)
- [Security model](docs/security.md)
- [Integrations and discovery](docs/integrations.md)
- [Development guide](docs/development.md)
- [Translation guide](docs/translating.md)

The privacy page explains that search terms are sent in request URLs and may appear in request logs. Theme, language, and consent preferences are stored in first-party cookies; preferences can be saved, shared, or reset.

Server-rendered pages include `/`, `/quotes`, `/server/about`, `/server/help`, `/server/help/mcp`, `/server/preferences`, `/server/preferences/tokens`, `/server/preferences/sync`, `/server/security`, `/server/security/policy`, `/server/security/thanks`, `/server/security/report/{tracking_id}`, `/server/contact`, `/server/privacy`, `/server/terms`, `/server/dpo`, `/server/docs/swagger`, and `/server/docs/graphql`. `/server/healthz` is a health endpoint, and `/server` redirects to `/server/about`.

## Development

Go builds and tests run in the `casjaysdev/go:latest` container through the Makefile; do not run Go directly on the host.

```sh
make dev
make test
make local
make build
```

The Makefile builds server and CLI binaries for Linux, macOS, Windows, and FreeBSD on amd64 and arm64. `tests/run_tests.sh` runs the Docker-only basic tier for binary, HTTP, and CLI checks, including help output after renaming both binaries. It exits with status 69 when Docker is unavailable. `tests/test_content_negotiation.sh` separately checks public HTML, API (including `/api/autodiscover`), and fixed discovery routes. Incus service lifecycle and browser E2E checks are separate manual tiers:

```sh
./tests/run_tests.sh
./tests/docker.sh
./tests/test_content_negotiation.sh
./tests/incus.sh
./tests/e2e.sh
```

See [Development](docs/development.md) for prerequisites and execution details.

## Disclaimer

This software is provided "as is" without warranty of any kind. Use it at your own risk.

- **No Warranty:** The authors are not responsible for damages, data loss, or issues arising from use of the software.
- **Third-Party Content:** Quote text remains the property of its respective rights holders.
- **Third-Party Services:** The server self-update command uses GitHub Releases and is subject to its terms. CLI update artifacts are provisioned by the operator or deployment package.
- **Security:** No software is guaranteed to be free of vulnerabilities.
- **Production Use:** Evaluate the software and configure the service for your deployment before production use.

By using this software, you acknowledge that you have read and understood this disclaimer.

## License

The application is MIT licensed. See [LICENSE.md](LICENSE.md).
