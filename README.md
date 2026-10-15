# Anime Quotes

Anime Quotes is a public, read-only Go service for a curated collection of anime quotes. The dataset is embedded in each server binary. No account, API key, database, or runtime external service is required.

## Features

- Server-rendered browser home and searchable quote collection
- Random quote and complete collection REST endpoints
- Case-insensitive search by quote text, anime title, and character
- Read-only GraphQL API and browser query explorer
- OpenAPI 3.1 JSON contract and API documentation page
- Companion `anime-cli` client for random, list, and search operations
- Embedded quote data and PWA manifest/service worker
- Anonymous access with request limits; no write endpoints or user accounts

## Run with Docker

The production compose file is `docker/docker-compose.yml`. It binds to the Docker bridge address on port 64580 and stores operator configuration and runtime data in the compose directory's `volumes/` paths. Review the port and volume settings before deployment.

```sh
docker compose -f docker/docker-compose.yml up -d
```

The service listens on port 80 in the container. Configure the generated `server.yml` under `/config/apimgr/anime/` or supply the `CONFIG_DIR`, `DATA_DIR`, and `LOGS_DIR` environment variables.

## Server binary

```sh
anime --help
anime --version
anime --port 8080 --address 127.0.0.1
```

The server generates its configuration on first start. Useful environment overrides are `PORT`, `ADDRESS`, `CONFIG_DIR`, `DATA_DIR`, and `LOGS_DIR`. `--status` checks `/api/v1/server/healthz` on the configured listen port.

## HTTP surfaces

| Surface | Route | Behavior |
|---|---|---|
| Home | `/` | Server-rendered random quote and service overview |
| Quote browser | `/quotes` | HTML search form; accepts `q`, `anime`, and `character` |
| Random quote | `GET /api/v1/random` | One random quote |
| Quote collection | `GET /api/v1/quotes` | All quotes; optional `q`, `anime`, and `character` filters |
| Health page | `/server/healthz` | HTML health page |
| Health API | `/api/v1/server/healthz` | JSON health response |
| OpenAPI | `/api/v1/server/swagger` | OpenAPI 3.1 JSON; `/api/swagger` is an alias |
| GraphQL | `/api/v1/server/graphql` | Read-only GraphQL GET or JSON POST; `/api/graphql` is an alias |
| API documentation | `/server/docs/swagger` | Route reference and OpenAPI link |
| GraphQL explorer | `/server/docs/graphql` | Browser query form |
| Informational pages | `/server/about`, `/server/help`, `/server/privacy`, `/server/terms` | Public HTML pages |

GraphQL fields are `randomQuote` and `quotes(q, anime, character)`. Quote fields are `anime`, `character`, and `quote`.

## CLI client

The CLI requires an explicit server URL because the project does not declare an official hosted site. You can also set `ANIME_SERVER_PRIMARY`.

```sh
anime-cli --server https://anime.example random
anime-cli --server https://anime.example --output json list
anime-cli --server https://anime.example search "fullmetal alchemist"
```

Use `--output text` or `--output json`. The CLI only calls public read-only API routes.

## Development

Go and project validation commands run inside `casjaysdev/go:latest` through the Makefile; do not run Go directly on the host.

```sh
make dev
make test
make local
make build
```

The Makefile builds static server and client binaries for Linux, macOS, Windows, and FreeBSD on amd64 and arm64. `make docker` builds and pushes the multi-platform image to the configured registry. `make clean` removes generated build outputs.

## Documentation

Detailed user, API, configuration, security, integration, and development documentation is in `docs/` and built with MkDocs for ReadTheDocs.

## License

The application is MIT licensed. See [LICENSE.md](LICENSE.md). Quote text remains the property of its respective rights holders.
