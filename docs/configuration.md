# Configuration

The server creates `server.yml` on first run under its configuration directory, generates an operator token, and selects an available port in the `64000`–`64999` range. The default directories are selected by `src/paths`; override them with `--config`/`CONFIG_DIR`, `--data`/`DATA_DIR`, `--cache`/`CACHE_DIR`, `--log`/`LOG_DIR`, `--backup`/`BACKUP_DIR`, and `--pid`/`PID_FILE`. `DATABASE_DIR` can select a database directory.

Server listen settings can be overridden in this order: command-line flags, environment, then `server.yml`. The server uses `PORT` for the port and `LISTEN` for the listen address. Outside the container, first-run configuration selects an available port; the container listens on port `80`.

## Configuration sections

The generated `server.yml` contains these sections:

| Section | Settings |
|---|---|
| `server` | `port`, `token`, `fqdn`, `address`, `baseurl`, `mode` |
| `server.database` | `driver`, `url`, and connection-pool limits |
| `server.update` | release `branch`, `auto_install`, `defer_days`, and `cli_min_version` |
| `server.schedule` | scheduler enablement and notification cadence |
| `server.metrics` | enablement, endpoint, and system/application metric switches |
| `server.logging` | access log format and log level |
| `web-ui` | `theme`, logo, favicon, announcements, and notification enablement |
| `web-robots` | lists of paths allowed or denied to crawlers |
| `web-security` | admin contact and CORS setting |

The config decoder rejects unknown fields. Keep the generated token private; it is an operator credential. Public read-only quote routes do not require it.

The CLI needs a server URL from `--server`, `ANIME_SERVER_PRIMARY`, or its `cli.yml` profile. No official service host is configured in this repository. The CLI stores configuration in the invoking user's config directory; do not put credentials or private data in a shared system directory.
