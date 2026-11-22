# Integrations and discovery

## Public interfaces

- REST API and OpenAPI JSON: `/api/v1/server/swagger` (`/api/swagger` alias); see [API reference](api.md).
- Read-only GraphQL: `/api/v1/server/graphql` (`/api/graphql` and `/graphql` aliases).
- MCP Streamable HTTP: `/api/v1/server/mcp` (`/api/mcp` and `/mcp` aliases). Each request requires an owner Bearer token. The guide at `/server/help/mcp` describes supported clients and tools.
- CLI discovery: `/api/autodiscover` reports supported release metadata; `/cli/binaries/{file}` serves a supported CLI artifact. The CLI verifies the artifact checksum.
- Contact and security reporting: `/api/v1/server/contact` and `/api/v1/server/security`; see [Security](security.md).

## Discovery files

| Route | Purpose |
|---|---|
| `/robots.txt` | Public crawler rules and absolute sitemap URL |
| `/sitemap.xml` | Public HTML pages allowed by the crawler rules |
| `/feed.xml` | Atom 1.0 feed |
| `/rss.xml` | RSS 2.0 feed |
| `/llms.txt`, `/.well-known/llms.txt` | API, MCP, and capability information for AI clients |
| `/.well-known/security.txt` | RFC 9116 security contacts, policy, expiry, and optional encryption key |
| `/.well-known/pgp-key.asc` | Public security-report encryption key when configured |
| `/manifest.json`, `/sw.js` | PWA manifest and service worker |

The canonical security file is `/.well-known/security.txt`; `/security.txt` is not served. Discovery URLs are generated for the request origin. Sitemap and feed URLs are absolute. The feeds are valid with no entries when the project has no time-stamped public quote records.

## External services

No identity provider, payment service, or runtime quote source is enabled. Quotes are embedded in the server binary. The explicit server update command and the CLI discovery/download routes use GitHub Releases; normal quote and health requests do not call GitHub.

The production Docker Compose deployment can expose an optional Tor hidden service. Its address appears on `/server/help` after Tor publishes it. Set `ENABLE_TOR=false` to disable the service.
