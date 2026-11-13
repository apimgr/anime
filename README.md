# Anime Quotes

[![CI](https://github.com/apimgr/anime/actions/workflows/ci.yml/badge.svg)](https://github.com/apimgr/anime/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/apimgr/anime)](https://github.com/apimgr/anime/releases)
[![License](https://img.shields.io/github/license/apimgr/anime)](LICENSE.md)

## About

Anime Quotes is a public, read-only Go application for browsing a curated collection of anime quotes. Quote data is embedded in the server binary. Visitors can use the server-rendered site, versioned REST API, read-only GraphQL API, or companion CLI. No user account or API key is required.

## Features

- Random quote retrieval and full collection browsing
- Case-insensitive search by quote text, anime title, or character
- Server-rendered, mobile-friendly quote browser
- REST API with OpenAPI documentation
- Read-only GraphQL API and browser explorer
- Companion `anime-cli` for random, list, and search operations
- PWA manifest and service worker
- Optional Tor hidden service in the production Docker Compose deployment
- Anonymous access with request limits and no write API

## Production

### Docker Compose

Review `docker/docker-compose.yml`, especially its published port and volume paths, then start the service:

```sh
docker compose -f docker/docker-compose.yml up -d
```

The service listens on port 80 in the container. Configuration is stored at `/config/anime/server.yml`; data and logs are under `/data/anime` and `/data/log/anime`. The compose file binds host port `64580` on the Docker bridge address.
Tor is enabled in the production Compose configuration. Its onion address is created in the Tor data volume and shown on `/server/help` after the hidden service is published.

### Binary

Download the matching server artifact from [GitHub Releases](https://github.com/apimgr/anime/releases), then run:

```sh
anime --help
anime --version
anime --port 8080 --address 127.0.0.1
```

The server creates `server.yml` on first run, generates an operator token, and selects an available port. For a systemd service, run `anime --service --install` and follow with `systemctl start anime` and `systemctl enable anime` as appropriate for the host.

## Client

Install the matching `anime-cli` artifact from [GitHub Releases](https://github.com/apimgr/anime/releases). The client needs an explicit server URL because this repository does not configure an official hosted service:

```sh
anime-cli --server https://anime.example random
anime-cli --server https://anime.example --output json list
anime-cli --server https://anime.example search "fullmetal alchemist"
```

`--server` overrides `ANIME_SERVER_PRIMARY`, which overrides the selected CLI profile. The client only calls public read-only API routes.

## Configuration

The server reads `server.yml` from its config directory. Directory overrides are `CONFIG_DIR`, `DATA_DIR`, `CACHE_DIR`, `LOG_DIR`, `BACKUP_DIR`, `PID_FILE`, and `DATABASE_DIR`; the corresponding CLI flags include `--config`, `--data`, `--cache`, `--log`, `--backup`, and `--pid`. `PORT` and `LISTEN` provide server listen defaults, and `--port`/`--address` override them.

See [Configuration](docs/configuration.md) for the server settings, defaults, first-run behavior, and CLI profile details.

## API

| Method | Route | Description |
|---|---|---|
| GET | `/api/v1/random` | Return one random quote |
| GET | `/api/v1/quotes` | List or search quotes with optional `q`, `anime`, and `character` filters |
| GET | `/api/v1/server/healthz` | Bare JSON health object; HTTP 503 when unhealthy |
| GET | `/api/v1/stats` | Runtime and collection statistics |
| GET | `/api/v1/server/swagger` | OpenAPI 3.1 JSON |
| GET, POST | `/api/v1/server/graphql` | Read-only GraphQL query endpoint |

Plain text forms are available by adding `.txt` to the random, quotes, health, and stats routes. `/api/healthz`, `/api/swagger`, and `/api/graphql` are direct aliases. The HTML browser and standard pages are `/`, `/quotes`, `/server/about`, `/server/help`, `/server/healthz`, `/server/privacy`, `/server/terms`, `/server/docs/swagger`, and `/server/docs/graphql`. `/server` redirects to `/server/about`.

See the [API reference](docs/api.md) for filters, response formats, route aliases, and GraphQL fields.

## Other

- [Installation](docs/installation.md)
- [CLI reference](docs/cli.md)
- [Security model](docs/security.md)
- [Integrations](docs/integrations.md)
- [Development guide](docs/development.md)
- [Translation guide](docs/translating.md)

The privacy page explains that search terms are sent in request URLs and may appear in request logs. Browser theme preference is stored in local storage.

## Development

Go builds and tests run in `casjaysdev/go:latest` through the Makefile; do not run Go directly on the host.

```sh
make dev
make test
make local
make build
```

The Makefile builds server and CLI binaries for Linux, macOS, Windows, and FreeBSD on amd64 and arm64. `make docker` builds and pushes the multi-platform image to the configured registry. `make clean` removes generated build outputs.

The executable scripts in `tests/` cover binary and HTTP behavior separately from Go unit tests. They keep build/runtime data in a project-scoped OS temporary directory and remove it on exit:

```sh
./tests/run_tests.sh
./tests/docker.sh
./tests/incus.sh
./tests/e2e.sh
```

`run_tests.sh` prefers Incus when available and otherwise runs Docker integration checks. The browser suite is on-demand and is not part of the commit gate. See [Development](docs/development.md) for prerequisites and details.

## Disclaimer

This software is provided "as is" without warranty of any kind. Use at your own risk.

- **No Warranty**: The authors are not responsible for damages, data loss, or issues arising from use of this software.
- **Third-Party Content**: Quote text remains the property of its respective rights holders.
- **Third-Party Services**: The explicit update command and CLI release discovery use GitHub Releases and are subject to its terms.
- **Security**: No software is guaranteed to be free of vulnerabilities.
- **Production Use**: Evaluate the software and configure the service for your deployment before production use.

By using this software, you acknowledge that you have read and understood this disclaimer.

## License

The application is MIT licensed. See [LICENSE.md](LICENSE.md).
