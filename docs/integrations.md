# Integrations

- REST API: see [API reference](api.md).
- OpenAPI JSON: `/api/v1/server/swagger` (`/api/swagger` alias).
- GraphQL: `/api/v1/server/graphql` (`/api/graphql` alias).
- CLI: see [CLI reference](cli.md).
- Public discovery: robots.txt, PWA manifest, service worker, and security.txt are served by the application.

No identity provider, webhook, payment service, or runtime quote source is enabled. The embedded quote collection is the only data source. The server's explicit update command checks GitHub Releases, and the CLI's discovery and binary download routes use GitHub Releases to locate verified CLI updates.

The server also exposes `/api/autodiscover` for CLI release metadata and `/cli/binaries/{name}` for supported CLI download redirects. The CLI verifies downloaded artifacts against the published SHA-256 checksums. Ordinary quote and health requests do not contact GitHub.
