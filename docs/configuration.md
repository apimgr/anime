# Configuration

The server creates `server.yml` on first run under its configuration directory, generates an operator token, and selects an available port in the `64000`–`64999` range. Encryption material is stored separately in `secrets.yml`; keep both files private. The default directories are selected by `src/paths`; override them with `--config`/`CONFIG_DIR`, `--data`/`DATA_DIR`, `--cache`/`CACHE_DIR`, `--log`/`LOG_DIR`, `--backup`/`BACKUP_DIR`, and `--pid`/`PID_FILE`. `DATABASE_DIR` can select a database directory.

Server listen settings can be overridden in this order: command-line flags, environment, then `server.yml`. The server uses `PORT` for the port and `LISTEN` for the listen address. Outside the container, first-run configuration selects an available port; the container listens on port `80`.

## Configuration sections

The generated `server.yml` contains these sections:

| Section | Settings |
|---|---|
| `server` | `port`, operator `token`, `fqdn`, `address`, `baseurl`, and `mode` |
| `server.database` | `driver`, `url`, and `pool.max_open`, `pool.max_idle`, `pool.max_lifetime`, and `pool.max_idle_time` |
| `server.update` | release `branch`, `auto_install`, `defer_days`, and `cli_min_version` |
| `server.schedule` | scheduler `enabled` and notification cadence |
| `server.metrics` | `enabled`, `endpoint`, `include_system`, and `include_app` |
| `server.logging` | `access_format` and `level` |
| `server.contact` | Public `admin`, `security`, `abuse`, `dpo`, and `general` email addresses; DPO page uses the DPO email, then the general email, and never publishes the admin address |
| `server.notifications.email` | SMTP host, port, username, TLS mode, and sender name/address. Supply the SMTP password through `SMTP_PASSWORD` rather than `server.yml` |
| `server.pages.contact` | Contact page enablement, CAPTCHA mode, and site key |
| `web-ui` | `theme`, logo, favicon, and announcement enablement/content |
| `web-robots` | `allow` and `deny` path lists for crawlers |
| `web-security` | Admin contact and CORS setting |

The config decoder rejects unknown fields. Keep the generated token private; it is an operator credential. Public read-only quote routes do not require it. SMTP environment overrides are `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_TLS`, `SMTP_FROM_NAME`, and `SMTP_FROM_EMAIL`; set them in the service/container environment and protect them as credentials where applicable.

The CLI needs a server URL from `--server`, `ANIME_SERVER_PRIMARY`, or its `cli.yml` profile. No official service host is configured in this repository. The CLI stores configuration in the invoking user's config directory; do not put credentials or private data in a shared system directory.
