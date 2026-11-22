# API reference

Quote, info, and statistics routes are public and read-only. The preferences endpoint only reads or updates browser-local theme and language cookies; it does not change server configuration or quote data. JSON success envelopes use `ok: true` and a `data` value. Health endpoints return the bare health object for monitoring probes, and GraphQL returns its standard `data` and `errors` fields. Validation failures use RFC 7807 problem JSON. No authentication or API key is needed.

| Method | Route | Description |
|---|---|---|
| GET | `/api/v1` | API index and supported routes |
| GET | `/api` | Unversioned API index alias |
| GET | `/api/v1/random` | One random quote |
| GET | `/api/v1/quotes` | List and search quotes |
| GET | `/api/v1/server/healthz` | Bare JSON health object; HTTP 503 when unhealthy |
| GET | `/api/v1/server/preferences` | Read effective browser theme and language preferences |
| POST | `/api/v1/server/preferences` | Save supported browser-local preferences |
| GET | `/api/v1/server/preferences/sync` | Export current browser preferences as a URL and share code |
| GET | `/api/v1/server/preferences/sync/import` | Apply supported preferences from a share URL, query, or code |
| GET | `/api/v1/server/about` | About information |
| GET | `/api/v1/server/help` | Usage guidance |
| GET | `/api/v1/server/privacy` | Privacy information |
| GET | `/api/v1/server/terms` | Terms information |
| GET | `/api/v1/server/dpo` | Configured DPO or general privacy contact |
| GET | `/api/v1/server/security` | Security reporting metadata and policy links |
| GET | `/api/v1/server/security/policy` | Disclosure policy text |
| GET | `/api/v1/server/security/thanks` | Security acknowledgments |
| GET | `/api/v1/server/healthz.txt` | Health as plain text; HTTP 503 when unhealthy |
| GET | `/api/v1/stats` | Runtime and collection statistics |
| GET | `/api/v1/stats.txt` | Statistics as plain text |
| GET | `/api/v1/server/swagger` | OpenAPI 3.1 JSON |
| GET or POST | `/api/v1/server/graphql` | Read-only GraphQL |
| GET | `/api/v1/random.txt` | Random quote as plain text |
| GET | `/api/v1/quotes.txt` | Quote collection as plain text |
| GET | `/api/healthz` | Direct alias for the current health endpoint |
| GET | `/api/swagger` | Direct alias for the current OpenAPI document |
| GET or POST | `/api/graphql` and `/graphql` | Direct aliases for the read-only GraphQL endpoint |
| GET | `/api/autodiscover` | CLI release discovery metadata |
| GET | `/cli/binaries/{file}` | CLI release artifact download |

The quote list accepts optional case-insensitive query parameters `q`, `anime`, and `character`. Each value is limited to 256 characters. `q` searches quote text, anime, and character; `anime` and `character` narrow their respective fields. The HTML collection at `/quotes` uses the same filters.

`/api/healthz`, `/api/swagger`, and `/api/graphql` are direct aliases for the corresponding current API handlers. `/graphql` is also a direct GraphQL alias. The API root also supports `/api` as an unversioned index. The browser reference pages are `/server/docs/swagger` and `/server/docs/graphql`.

The CLI discovery endpoint reads release metadata from GitHub Releases and caches it in memory for five minutes. The binary endpoint serves supported release artifacts; the CLI verifies each downloaded artifact against its published checksum. These routes are separate from normal quote and health requests, which do not call external services.

GraphQL supports `randomQuote { anime character quote }` and `quotes(q: ..., anime: ..., character: ...) { anime character quote }`. POST requests use `application/json` with a `query` string. GET requests provide the query in a `query` URL parameter. GraphQL results use the standard GraphQL `data` and `errors` fields.

The public server pages include `/server/about`, `/server/help`, `/server/healthz`, `/server/preferences`, `/server/preferences/sync`, `/server/privacy`, `/server/terms`, `/server/dpo`, `/server/security`, `/server/security/policy`, and `/server/security/thanks`. Their JSON counterparts are available at the matching `/api/v1/server/...` routes. The DPO page uses `server.contact.dpo.email` and falls back to `server.contact.general.email`; it never exposes the admin address. The security overview contains the current expiry and private reporting channel; the report form is not enabled. `/server/preferences/sync/import` is an action route that applies a shared preference set and redirects. `/server` redirects to `/server/about`.
