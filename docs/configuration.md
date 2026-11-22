# Configuration

Configuration is file-backed. The server merges built-in defaults, `server.yml`, ordered files in `yaml.d/`, and `secrets.yml` (last). The database stores application state such as anonymous owners and token records; it is not an operator configuration source.

## Files and permissions

| File | Purpose |
|---|---|
| `server.yml` | Node identity and listen settings, database, cache, TLS, proxy, and cluster settings |
| `yaml.d/branding.yml` | Branding, SEO, and web UI settings |
| `yaml.d/web.yml` | Web security, crawler rules, CORS, and URL behavior |
| `yaml.d/pages.yml` | Page enablement and contact-form policy |
| `yaml.d/contact.yml` | Public contact destinations and webhooks |
| `yaml.d/privacy.yml` | Privacy, compliance, and tracking settings |
| `yaml.d/security.yml` | Security, rate-limit, GeoIP, token, and data settings |
| `yaml.d/logs.yml` | Log configuration |
| `yaml.d/metrics.yml` | Metrics configuration |
| `yaml.d/notifications.yml` | SMTP and notification configuration |
| `yaml.d/maintenance.yml` | Scheduler, update, backup, and restore settings |
| `yaml.d/app.yml` | Project-specific settings not assigned to another area file |
| `secrets.yml` | Credentials only; merged last |

All files are YAML. Drop-ins are merged in byte-wise filename order; maps merge recursively, while a later scalar or list replaces the earlier value. Hidden files, backups, non-YAML files, and subdirectories are ignored. `yaml.d/` uses mode `0700`; configuration files use mode `0600`. Unparseable files are ignored and logged. Invalid values should fall back to their defaults with a warning.

On startup, legacy settings in `server.yml` are migrated into their assigned area files. The original is backed up as `server.yml.bak`; writes use private permissions. If a value already exists in its destination area file, that value takes precedence.

## First run and paths

The server creates its configuration directory and operator token on first run. The generated operator token is stored in `secrets.yml`, not `server.yml`; keep the secrets file private. If no listen port is supplied, the server selects an available port in the configured range. The container listens on port `80`.

Default directories are selected by the application. Override them with `--config`/`CONFIG_DIR`, `--data`/`DATA_DIR`, `--cache`/`CACHE_DIR`, `--log`/`LOG_DIR`, `--backup`/`BACKUP_DIR`, `--pid`/`PID_FILE`, and `DATABASE_DIR`. `PORT` and `LISTEN` provide listen defaults. Command-line options take priority over environment and file settings.

## Common settings

| Setting | Area file | Purpose |
|---|---|---|
| `server.port`, `server.address`, `server.fqdn`, `server.mode`, `server.baseurl` | `server.yml` | Listen endpoint, public identity, operating mode, and path prefix |
| `server.database.driver`, `.url`, `.pool.*` | `server.yml` | SQLite or libSQL connection and pool |
| `server.tokens.max_linked`, `.cookie_max_age`, `.idle_purge_days` | `yaml.d/security.yml` | Owner-token policy |
| `server.rate_limit.read`, `.strict` | `yaml.d/security.yml` | Public and sensitive-operation limits |
| `server.pages.contact.enabled`, `.captcha`, `.antispam.*` | `yaml.d/pages.yml` | Contact/report form and abuse controls |
| `server.contact.*` | `yaml.d/contact.yml` | DPO, general, abuse, admin, and security contacts |
| `server.notifications.email.smtp.*`, `.from.*` | `yaml.d/notifications.yml` | SMTP connection and sender settings |
| `server.security.reports.retention_days` | `yaml.d/security.yml` | Closed-report retention |
| `web.security.report_url`, `.contact`, `.policy`, `.keyservers`, `.publish_pgp_key` | `yaml.d/web.yml` | Security.txt, disclosure policy, and key publication |
| `web.robots.allow`, `.deny` | `yaml.d/web.yml` | Crawler policy and sitemap inclusion |
| `web.llms.enabled`, `.include_endpoints`, `.include_schemas`, `.custom_sections` | `yaml.d/web.yml` | LLM discovery output |
| `server.update.branch`, `.auto_install`, `.defer_days`, `.cli_min_version` | `yaml.d/maintenance.yml` | Release channel and update behavior |

## Branding and page content

`yaml.d/branding.yml` holds `server.branding`, `server.seo`, and `web.ui`. Branding fields include the site title, tagline, description, logo, and favicon. SEO settings control canonical URLs, metadata, social cards, verification tags, sitemap limits, and feed titles and item counts.

Operator-authored page content is optional and uses `server.branding.content`:

```yaml
server:
  branding:
    content:
      about:
        format: md
        body:
          default: |
            Run by the Example team.
          fr: |
            Géré par l'équipe Example.
      terms:
        format: md
        body: |
          Terms maintained by the service operator.
      footer:
        format: html
        body: '<p>Powered by <strong>Example</strong></p>'
```

`header` and `footer` accept sanitized HTML only. `about`, `privacy`, and `terms` accept `md`, `html`, or `txt`; Markdown and HTML are sanitized before rendering. A localized `body` map selects the current language, then `default`; a plain string applies to every language. Each body is limited to 256 KiB. Operator content cannot link to `/server/contact`. About, privacy, and terms content extends the built-in page, except terms content replaces the built-in terms. The footer entry replaces only its branding block; the shared application footer remains.

Privacy and consent settings live in `yaml.d/privacy.yml` under `server.privacy`. `data.sold` controls the privacy notice, consent wording, and CCPA opt-out controls. The same area holds data-sharing descriptions, retention text, cookie categories, consent-banner placement and labels, and manually declared third-party services. Essential cookies are always enabled. Consent messages and labels fall back to built-in translations when their configured values are empty or equal to the built-in English defaults.

```yaml
server:
  privacy:
    data:
      sold: false
      stored_on_server: true
      sharing:
        - condition: user_initiated
          when: When a visitor submits a report
          data: The name, reply address, and report content supplied by the visitor.
    retention:
      period: Operational log retention is controlled by the deployment configuration.
      export_available: true
      deletion_available: true
    consent:
      show_until_acknowledged: true
      position: bottom
      show_preferences: true
      policy:
        text: Privacy policy
        url: /server/privacy
```

Sensitive values belong in `secrets.yml`, including `server.token`, `server.security.encryption_key`, SMTP passwords, CAPTCHA secret keys, and webhook credentials. Use environment variables only where the deployment explicitly supports them; SMTP overrides are `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_TLS`, `SMTP_FROM_NAME`, and `SMTP_FROM_EMAIL`.

## CLI profiles

The companion client reads a user-scoped `cli.yml` profile. Select a server with `--server`, `ANIME_SERVER_PRIMARY`, or the profile's `server.primary` value. Keep client configuration private and do not place credentials or private data in shared system directories. See [CLI reference](cli.md) for output, cache, logging, and shell integration options.
