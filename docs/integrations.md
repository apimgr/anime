# Integrations and discovery

## Public interfaces

- REST API and OpenAPI JSON: `/api/v1/server/swagger` (`/api/swagger` alias); see [API reference](api.md).
- Read-only GraphQL: `/api/v1/server/graphql` (`/api/graphql` and `/graphql` aliases).
- MCP Streamable HTTP: `/api/v1/server/mcp` (`/api/mcp` and `/mcp` aliases). Each request requires an owner Bearer token. The guide at `/server/help/mcp` describes supported clients and tools.
- CLI discovery: `/api/autodiscover` reports `cli_versions` and `cli_min_version` in the standard `ok`/`data` envelope; `/cli/binaries/{file}` serves a supported CLI artifact. The CLI checks discovery on each invocation, downloads updates over HTTPS, verifies SHA-256, atomically replaces its executable, and restarts it. Use `anime-cli --update check` to check explicitly or `anime-cli --update yes` to install.
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

The service worker caches versioned static assets and only public HTML
responses explicitly marked with `X-Offline-Cache: public`. Owner, token,
administration, and form pages are excluded; offline navigation shows a
localized offline page instead of stale private content. A new worker waits
until the visitor accepts the in-page update prompt, then activates and reloads
the page. Supported browsers can install the app from the footer button; on
iOS, the button explains how to add it from the browser's share menu.

The canonical security file is `/.well-known/security.txt`; `/security.txt` is not served. Discovery URLs are generated for the request origin. Sitemap and feed URLs are absolute. The feeds are valid with no entries when the project has no time-stamped public quote records.

## External services

No identity provider, payment service, or runtime quote source is enabled. Quotes are embedded in the server binary. The explicit server update command uses GitHub Releases. CLI discovery and downloads use assets provisioned by the operator (or image/package build) in `{data_dir}/cli` by default; the server does not fetch CLI releases. Normal quote and health requests do not call GitHub.

The application starts Tor when its binary is available, and starts I2P according to `server.i2p.enabled` (enabled by default in container mode). Their addresses and health appear on `/server/help` after publication. Tor and I2P are managed by the application; there is no `ENABLE_TOR` toggle.
