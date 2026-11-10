# API reference

All API routes are public and read-only. JSON endpoints return `application/json` success envelopes with `ok: true` and a `data` value; validation failures use RFC 7807 problem JSON. No authentication or API key is needed.

| Method | Route | Description |
|---|---|---|
| GET | `/api/v1` | API index and supported routes |
| GET | `/api/v1/random` | One random quote |
| GET | `/api/v1/quotes` | All quotes |
| GET | `/api/v1/server/healthz` | Service health |
| GET | `/api/v1/stats` | Runtime and collection statistics |
| GET | `/api/v1/server/swagger` | OpenAPI 3.1 JSON |
| GET or POST | `/api/v1/server/graphql` | Read-only GraphQL |

The quote list accepts optional case-insensitive query parameters `q`, `anime`, and `character`. `q` searches quote text, anime, and character; `anime` and `character` narrow their respective fields. The HTML collection at `/quotes` uses the same filters.

`/api/swagger` and `/api/graphql` are aliases for the corresponding versioned API handlers. The browser reference pages are `/server/docs/swagger` and `/server/docs/graphql`.

GraphQL supports `randomQuote { anime character quote }` and `quotes(q: ..., anime: ..., character: ...) { anime character quote }`. POST requests use JSON with a `query` string. GET requests provide the query in a `query` URL parameter.
