## Project description

Anime is a full-stack Go web application serving anime quotes from a curated collection. It provides random quote retrieval and full quote listing through a versioned REST API, GraphQL endpoint, and a server-side rendered web UI for browsing and discovering quotes. The dataset is embedded in the binary at build time. Deployed as a single self-contained static binary.

## Project variables

project_name: anime
project_org: apimgr
internal_name: anime
internal_org: apimgr
app_name: Anime Quotes API
repo: https://github.com/apimgr/anime
license: MIT
binary: anime
client_binary: anime-cli
owner_token: anime_owner_token_4mxwkxfqGg

## Business logic

### Product scope & non-goals

**In scope:**
- Serving anime quotes via REST API and web UI
- Random quote endpoint and full listing endpoint
- Filtering/searching by quote text, anime title, or character name
- Public JSON API responses with a consistent success envelope
- Full web frontend (server-side Go templates, dark/light/auto theme, PWA, mobile-first)
- Server pages: `/server/about`, `/server/help`, `/server/healthz`, `/server/privacy`, `/server/terms`, `/server/dpo`
- Server security reporting pages, contact reporting, token management, and MCP connection help required by the API baseline
- Informational JSON counterparts at `/api/{api_version}/server/{about,help,privacy,terms,dpo}`
- CLI client (`anime-cli`) for fetching quotes from the terminal
- OpenAPI/Swagger documentation at `/server/docs/swagger` and machine-readable OpenAPI JSON at `/api/{api_version}/server/swagger` (`/api/swagger` alias)
- GraphQL explorer at `/server/docs/graphql` and read-only GraphQL queries at `/api/{api_version}/server/graphql` (`/api/graphql` alias)
- CLI client commands for random quote retrieval, listing, and text search; the server URL is explicit because no official site is configured

**Non-goals:**
- No human user accounts, profiles, registration, or login. Anonymous owner tokens are still required by the API baseline for MCP access and token management.
- No admin web panel; operator settings are file-based using the configuration layout required by AI.md.
- No write/mutation API (quote data is read-only, embedded at build time)
- No anime episode/character database beyond what appears in quotes
- No paid tiers, no API keys, no rate-limited access tiers

### Roles & permissions

There are no human user roles or accounts. Public quote reads remain anonymous; owner-token holders manage their own anonymous credential and use the authenticated MCP endpoint. Operator actions use the operator token.

| Actor | Access |
|-------|--------|
| **Anonymous visitor (browser)** | Full read access to all web pages and API endpoints |
| **Anonymous API client (curl/CLI)** | Full read access to all API endpoints |
| **Server operator** | Configures server via `server.yml` only; no web management interface |

### Data model & sensitivity

**Quote record** (embedded at build time, no PII):

| Field | Type | Sensitivity |
|-------|------|-------------|
| `anime` | string — series title | Public |
| `character` | string — character name | Public |
| `quote` | string — quote text | Public |

Quote records contain no PII. Contact and security reports can contain a reporter's name, email address, reported URLs, and submitted report content. Security report bodies and researcher contact details are encrypted at rest as required by AI.md; public status pages expose only the permitted triage metadata. Owner records contain opaque identifiers and token metadata; raw owner tokens are never stored or logged.

The quote collection is embedded in the binary and is not stored in the runtime database. The server still uses its configured local SQLite database for application state required by the server baseline.

### Trust boundaries & external services

| Boundary | Trust level | Notes |
|----------|-------------|-------|
| Quote dataset (embedded at build) | Fully trusted | Static, embedded at compile time |
| Incoming HTTP requests | **Untrusted** | All query parameters validated |

Normal quote and health requests do not call external services. The explicit `--update` command and CLI discovery/download endpoints consult GitHub Releases; client metadata is cached in memory for five minutes, and downloads are checksum-verified by the CLI. Configured SMTP, CAPTCHA, webhook, and PGP keyserver integrations are external trust boundaries; failures must not expose submitted report contents or credentials.

### Threat model & abuse cases

**Primary assets:** service availability; the embedded quote collection; operator credentials and encryption keys; anonymous owner credentials; contact and security report content and researcher contact details.

**Attacker/abuser goals:**
- DoS via high-rate requests
- Bulk scraping of the full dataset
- Theft or replay of owner/operator tokens
- Spam, malicious links, or forged reports through public contact and security forms
- Exposure of researcher identity or vulnerability details

**Defenses:**
- Rate limiting on all endpoints
- Request size limits on all inputs
- Bearer-token validation, CSRF and origin checks, CAPTCHA, report encryption, and abuse controls on applicable forms
- There are no human accounts or passwords; anonymous owner tokens are credential-only identities, not user profiles.

**Non-threats (explicitly out of scope):**
- Admin panel compromise — no admin panel exists
- Account enumeration — no accounts exist

### Security decisions & exceptions

- **Public quote reads**: intentional and anonymous. The MCP endpoint requires the existing API Bearer token; owner tokens support anonymous credential management without creating human accounts.
- **CORS**: public data API supports cross-origin access by default without credentialed wildcard CORS; explicit origins are required for credentialed browser requests.

### API routes

All routes are public. Quote and informational data routes are read-only; the preferences endpoint only reads or updates the caller's theme and language cookies and never changes server configuration or quote data. JSON success responses use the `ok`/`data` envelope except the health endpoint, which returns its health document directly. Invalid search terms return an RFC 7807 problem response.

| Method | Path | Auth | Purpose and request/response |
|--------|------|------|-------------------------------|
| GET | `/api/v1` and `/api` | None | API index; returns the available route list. |
| GET | `/api/v1/random` | None | Return one random quote in the JSON success envelope. |
| GET | `/api/v1/quotes` | None | Return quotes; optional `q`, `anime`, and `character` filters, each up to 256 characters. |
| GET | `/api/v1/stats` | None | Return collection, runtime, and aggregate request statistics in the JSON success envelope. |
| GET | `/api/v1/server/{about,help,privacy,terms}` | None | Return the named informational page as JSON. |
| GET | `/api/v1/server/dpo` | None | Return the configured DPO or general privacy contact and request-specific privacy URL. |
| GET | `/api/v1/server/security` | None | Return private vulnerability-reporting details, current expiry, and policy and acknowledgments URLs. |
| GET | `/api/v1/server/security/policy` | None | Return the disclosure policy as Markdown. |
| GET | `/api/v1/server/security/thanks` | None | Return credited researchers; empty until disclosures are recorded. |
| GET | `/api/v1/server/healthz` | None | Return service health and public build/status information directly; HTTP 503 indicates unhealthy status. |
| GET, POST | `/api/v1/server/preferences` | None | Read or update browser-local `theme` (`dark`, `light`, `auto`) and `lang` (`en`) cookie values. Unknown or invalid preferences are reported as ignored. |
| GET | `/api/v1/server/preferences/sync` | None | Export effective theme and language preferences as a share URL and unpadded base64url code. |
| GET | `/api/v1/server/preferences/sync/import` | None | Apply supported browser preferences from direct query values or a `code`; consent and credentials are excluded. |
| GET | `/api/v1/server/swagger` | None | Return the OpenAPI 3.1 document. |
| GET, POST | `/api/v1/server/graphql` | None | Execute a read-only query. GET accepts a `query` parameter; POST accepts a JSON object containing `query`. |
| POST | `/api/v1/server/mcp` | None | Execute the public quote and service tools over MCP Streamable HTTP; `/api/mcp` and `/mcp` are direct aliases. |
| GET | `/api/v1/server/help/mcp` | None | Return the MCP connection guide and live public tool list. |
| GET | `/api/v1/{random,quotes,stats}.txt` | None | Return the corresponding result as plain text. |
| GET | `/api/v1/server/healthz.txt` | None | Return health status as plain text; HTTP 503 indicates unhealthy status. |
| GET | `/api/{healthz,swagger}` | None | Compatibility aliases for health JSON and the OpenAPI document. |
| GET, POST | `/api/graphql` and `/graphql` | None | Compatibility aliases for the read-only GraphQL endpoint. |
| GET | `/api/autodiscover` | None | Return release metadata used by the CLI to discover client downloads. |
| GET | `/cli/binaries/{file}` | None | Serve a supported CLI release artifact discovered through the update service; downloads are checksum-verified by the client. |

The browser pages are `/`, `/quotes`, `/server/about`, `/server/help`, `/server/help/mcp`, `/server/healthz`, `/server/preferences`, `/server/preferences/sync`, `/server/privacy`, `/server/terms`, `/server/dpo`, `/server/security`, `/server/security/policy`, `/server/security/thanks`, `/server/docs/swagger`, and `/server/docs/graphql`. `/server/preferences/sync` exports preferences; `/server/preferences/sync/import` is an action route that applies valid values and redirects. `/server` redirects to `/server/about`. Static/PWA routes are `/static/*`, `/robots.txt`, `/security.txt`, `/.well-known/security.txt`, `/manifest.json`, and `/sw.js`.
