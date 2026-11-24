# Anime Quotes

Anime Quotes serves a curated, embedded collection through a server-rendered web interface, REST API, GraphQL, and companion CLI. The dataset is read-only and public.

## Links

- [Repository](https://github.com/apimgr/anime)
- [ReadTheDocs documentation](https://apimgr-anime.readthedocs.io/)
- [License](https://github.com/apimgr/anime/blob/main/LICENSE.md)

On a running installation, the browser explorers are available at `/server/docs/swagger` and `/server/docs/graphql`.

## Runtime features

The server includes a persistent scheduler for maintenance and health jobs, authenticated Prometheus metrics, GeoIP and security dataset refreshes, verified backup and restore, SMTP operator notifications, TLS certificate renewal, log rotation, and staged server updates. The [configuration guide](configuration.md) documents each feature and its settings. [Installation](installation.md) covers supported system service managers and privilege setup.

Server-rendered pages are `/`, `/quotes`, `/server/about`, `/server/help`, `/server/help/mcp`, `/server/preferences`, `/server/preferences/tokens`, `/server/preferences/sync`, `/server/security`, `/server/security/policy`, `/server/security/thanks`, `/server/security/report/{tracking_id}`, `/server/contact`, `/server/privacy`, `/server/terms`, `/server/dpo`, `/server/docs/swagger`, and `/server/docs/graphql`. `/server/healthz` is a health endpoint; `/server` redirects to `/server/about`.

- [Installation](installation.md)
- [Configuration](configuration.md)
- [API reference](api.md)
- [CLI reference](cli.md)
- [Security](security.md)
- [Integrations](integrations.md)
- [Development](development.md)
- [Translation guide](translating.md)
