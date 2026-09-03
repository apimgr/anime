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

## Business logic

### Product scope & non-goals

**In scope:**
- Serving anime quotes via REST API and web UI
- Random quote endpoint and full listing endpoint
- Filtering/searching by anime title or character name
- Full web frontend (server-side Go templates, dark/light/auto theme, PWA, mobile-first)
- Server pages: `/server/about`, `/server/help`, `/server/healthz`, `/server/privacy`, `/server/terms`
- CLI client (`anime-cli`) for fetching quotes from the terminal
- OpenAPI/Swagger docs at `/api/{api_version}/server/swagger`
- GraphQL at `/graphql`

**Non-goals:**
- No user accounts, registration, or login of any kind
- No admin web panel (server configured via `server.yml` only)
- No write/mutation API (quote data is read-only, embedded at build time)
- No anime episode/character database beyond what appears in quotes
- No paid tiers, no API keys, no rate-limited access tiers

### Roles & permissions

There are no user roles. All endpoints are public and require no authentication.

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

No PII is stored or served.

### Trust boundaries & external services

| Boundary | Trust level | Notes |
|----------|-------------|-------|
| Quote dataset (embedded at build) | Fully trusted | Static, embedded at compile time |
| Incoming HTTP requests | **Untrusted** | All query parameters validated |

No external services are called at runtime.

### Threat model & abuse cases

**Primary assets:** the service itself (availability).

**Attacker/abuser goals:**
- DoS via high-rate requests
- Bulk scraping of the full dataset

**Defenses:**
- Rate limiting on all endpoints
- Request size limits on all inputs
- No user accounts eliminates credential stuffing and privilege escalation entirely

**Non-threats (explicitly out of scope):**
- Admin panel compromise — no admin panel exists
- Account enumeration — no accounts exist

### Security decisions & exceptions

- **No authentication on any endpoint**: intentional. Public read-only reference API. Rate limiting is the sole abuse-prevention mechanism.
- **All responses include `Access-Control-Allow-Origin: *`**: intentional. Public data API designed for cross-origin browser use.
