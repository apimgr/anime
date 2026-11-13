# API reference

All API routes are public and read-only. JSON endpoints return `application/json` success envelopes with `ok: true` and a `data` value; validation failures use RFC 7807 problem JSON. No authentication or API key is needed.

| Method | Route | Description |
|---|---|---|
| GET | `/api/v1` | API index and supported routes |
| GET | `/api` | Unversioned API index alias |
| GET | `/api/v1/random` | One random quote |
| GET | `/api/v1/quotes` | List and search quotes |
| GET | `/api/v1/server/healthz` | Service health |
| GET | `/api/v1/server/healthz.txt` | Health as plain text |
| GET | `/api/v1/stats` | Runtime and collection statistics |
| GET | `/api/v1/stats.txt` | Statistics as plain text |
| GET | `/api/v1/server/swagger` | OpenAPI 3.1 JSON |
| GET or POST | `/api/v1/server/graphql` | Read-only GraphQL |
| GET | `/api/v1/random.txt` | Random quote as plain text |
| GET | `/api/v1/quotes.txt` | Quote collection as plain text |

The quote list accepts optional case-insensitive query parameters `q`, `anime`, and `character`. Each value is limited to 256 characters. `q` searches quote text, anime, and character; `anime` and `character` narrow their respective fields. The HTML collection at `/quotes` uses the same filters.

`/api/healthz`, `/api/swagger`, and `/api/graphql` are direct aliases for the corresponding versioned API handlers. The API root also supports `/api` as an unversioned index. The browser reference pages are `/server/docs/swagger` and `/server/docs/graphql`.

GraphQL supports `randomQuote { anime character quote }` and `quotes(q: ..., anime: ..., character: ...) { anime character quote }`. POST requests use `application/json` with a `query` string. GET requests provide the query in a `query` URL parameter. GraphQL results use the standard GraphQL `data` and `errors` fields.

The public server pages are `/server/about`, `/server/help`, `/server/healthz`, `/server/privacy`, and `/server/terms`. `/server` redirects to `/server/about`.
