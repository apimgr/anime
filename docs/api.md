# API reference

Public quote reads and public information routes are anonymous. A primary owner Bearer token is required for token management; every MCP request requires a Bearer owner token. API routes ignore browser cookies. Token values are returned only by issue, restore, reveal, and rotation actions, never by list responses.

Successful REST responses use the documented `ok`/`data` envelope, except health responses and JSON-RPC, which use their own formats. Invalid input uses RFC 7807 problem JSON. GraphQL returns its standard `data` and `errors` fields.

## Routes

| Method | Route | Authentication | Description |
|---|---|---|---|
| GET | `/api/v1` and `/api` | None | API route index |
| GET | `/api/v1/server/swagger` and `/api/swagger` | None | OpenAPI 3.1 JSON document |
| GET, POST | `/api/v1/server/graphql`, `/api/graphql`, and `/graphql` | None | Read-only GraphQL query interface |
| POST | `/api/v1/server/mcp`, `/api/mcp`, and `/mcp` | Owner Bearer | MCP Streamable HTTP JSON-RPC endpoint |
| GET | `/api/v1/random` | None | One random quote |
| GET | `/api/v1/quotes` | None | List and search quotes using optional `q`, `anime`, and `character` filters |
| GET | `/api/v1/stats` | None | Runtime and collection statistics |
| GET | `/api/v1/server/{about,help,privacy,terms,dpo}` | None | Informational content and privacy contact |
| GET, POST | `/api/v1/server/contact` | None | Contact/report form metadata and submission |
| GET | `/api/v1/server/security` | None | Security contacts, policy, expiry, and key metadata |
| GET | `/api/v1/server/security/{policy,thanks}` | None | Disclosure policy and researcher acknowledgments |
| GET | `/api/v1/server/healthz` | None | Health status; HTTP 503 when unhealthy |
| GET | `/api/v1/server/auth/check` | Operator or owner Bearer | Side-effect-free machine check returning `kind: operator` or `kind: owner`; invalid tokens receive 401 |
| GET, POST | `/api/v1/server/preferences` | None | Read or update browser-local preferences |
| GET | `/api/v1/server/preferences/sync` | None | Export preferences as a share URL and code |
| GET | `/api/v1/server/preferences/sync/import` | None | Apply supported preferences from a share URL, query, or code |
| GET | `/api/v1/server/preferences/tokens` | None or owner Bearer | Token page data and masked token list |
| POST | `/api/v1/server/preferences/tokens/primary` | None | Issue a primary owner token |
| POST | `/api/v1/server/preferences/tokens/primary/{reveal,rotate}` | Primary owner Bearer | Reveal or rotate the primary token |
| DELETE | `/api/v1/server/preferences/tokens/primary` | Primary owner Bearer and confirmation of the token's final four characters | Revoke the owner and all of its tokens |
| POST | `/api/v1/server/preferences/tokens` | Primary owner Bearer | Create a linked token |
| POST | `/api/v1/server/preferences/tokens/{id}/{reveal,rotate}` | Primary owner Bearer | Reveal or rotate a linked token |
| PATCH | `/api/v1/server/preferences/tokens/{id}` | Primary owner Bearer | Rename a linked token or change its expiry |
| DELETE | `/api/v1/server/preferences/tokens/{id}` | Primary owner Bearer | Revoke a linked token |
| POST | `/api/v1/server/mcp` | Owner Bearer | MCP Streamable HTTP JSON-RPC endpoint |
| GET | `/api/v1/server/help/mcp` | None | MCP connection instructions and live tool list |
| GET | `/api/v1/server/swagger` | None | OpenAPI 3.1 JSON |
| GET, POST | `/api/v1/server/graphql` | None | Read-only GraphQL |
| GET | `/api/v1/{random,quotes,stats}.txt` | None | Plain-text responses |
| GET | `/api/v1/server/healthz.txt` | None | Health status as plain text |
| GET | `/server/metrics[/prometheus|/grafana|/loki]` and `/api/v1/server/metrics[/prometheus|/grafana|/loki]` | Matching metrics service Bearer token | Authenticated Prometheus metrics, Grafana dashboard, or sanitized Loki access stream; available when metrics are enabled |
| GET | `/api/metrics[/prometheus|/grafana|/loki]` | Matching metrics service Bearer token | Compatibility aliases for the metrics endpoints |
| GET | `/metrics[/prometheus|/grafana|/loki]` | Matching metrics service Bearer token | Root aliases when `server.metrics.root.enabled` is true |
| GET | `/api/autodiscover` | None | CLI release discovery metadata |
| GET | `/cli/binaries/{file}` | None | Supported CLI release artifact |
| GET | `/api/v1/debug/{endpoint}` and `/api/debug/{endpoint}` | Operator Bearer | Debug endpoints, only when debug routes are enabled |

The primary token is issued through `POST /api/v1/server/preferences/tokens/primary`. The web-only token actions are `POST /server/preferences/token/{create,restore,forget}` and `/server/preferences/tokens/{id}/{reveal,rotate,revoke}`. These browser actions use the secure owner cookie and CSRF protection; API token actions use Bearer headers. The consent action is `POST /server/consent`. The web page is `/server/preferences/tokens`.

## Quote filters and formats

The quote list accepts optional case-insensitive `q`, `anime`, and `character` query parameters, each up to 256 characters. `q` searches quote text, anime, and character; the other parameters narrow their corresponding fields. `/quotes` uses the same filters.

Plain-text routes are available for random quote, quote list, health, and statistics. JSON APIs can also be accessed using `/api/healthz`, `/api/swagger`, `/api/graphql`, and `/api/mcp`. GraphQL also has `/graphql`. MCP aliases are `/api/mcp` and `/mcp`. Metrics endpoint aliases are listed above; each requires its matching configured bearer token unless `server.metrics.auth.allow_unauthenticated` is enabled.

GraphQL supports `randomQuote { anime character quote }` and `quotes(q: ..., anime: ..., character: ...) { anime character quote }`. POST uses `application/json` with a `query` string; GET supplies `query` in the URL.

## Browser pages

The browser surface includes the quote pages, `/server/about`, `/server/help`, `/server/help/mcp`, `/server/healthz`, `/server/preferences` and its registered sub-pages, `/server/contact`, `/server/privacy`, `/server/terms`, `/server/dpo`, `/server/security` and its policy/thanks pages, `/server/security/report/{tracking_id}` (private report status, requiring its single-use status token), and the Swagger and GraphQL explorers. The `/server` route redirects to `/server/about`. State-changing token and consent routes and preference import routes are actions, not pages.

The CLI release discovery endpoint reads the operator-provisioned artifacts under `{data_dir}/cli`, including `version.txt` and `sha256.txt`. The binary endpoint serves a supported artifact only when its SHA-256 matches the manifest; the CLI verifies the same checksum after download. Server self-updates use GitHub Releases separately. Ordinary quote and health requests do not contact GitHub.
