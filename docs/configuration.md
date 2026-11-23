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

## Complete server settings index

The following list covers the server configuration fields. A field's YAML area is shown in the preceding file table; legacy values are accepted during migration from `server.yml` and then written to their assigned area. Secret values are loaded from `secrets.yml` last.

| YAML key or key group | Fields and behavior |
|---|---|
| `server` | `port`, `address`, `fqdn`, `baseurl`, and `mode` set the listener and public identity. `update.branch`, `update.auto_install`, `update.defer_days`, and `update.cli_min_version` control updates. `schedule.enabled` and `schedule.notifications` control in-process scheduled work. `metrics.enabled`, `metrics.endpoint`, `metrics.include_system`, and `metrics.include_app` control the metrics endpoint and exported measurements. `logging.access_format` and `logging.level` set access-log format and verbosity. |
| `server.database` | `driver`, `url`, and `pool.max_open`, `pool.max_idle`, `pool.max_lifetime`, `pool.max_idle_time` select SQLite/libSQL and connection-pool limits. |
| `server.i18n` | `default_language` and `available_languages` define the server locale fallback and enabled language set. The bundled catalogs currently support `en`, `es`, `fr`, `de`, `it`, and `ar`. |
| `server.rate_limit` | `read.requests`, `read.window`, `strict.requests`, and `strict.window` define the normal and sensitive-operation request limits. Windows are seconds. |
| `server.tokens` | `max_linked`, `cookie_max_age`, and `idle_purge_days` cap linked tokens, set the owner-cookie lifetime, and configure idle-owner cleanup. |
| `server.security` | `encryption_key` and `encryption_key_version` protect encrypted report data and key rotation. `reports.retention_days` controls closed security-report retention. Put the encryption key in `secrets.yml`. |
| `server.pages.contact` | `enabled`, `captcha`, `captcha_site_key`, `captcha_secret_key`, and `success_message` control the public report form and CAPTCHA. `antispam.honeypot`, `min_fill_seconds`, `max_age_seconds`, `per_ip`, `per_email`, `global`, `window`, `max_links`, `reject_markup`, `blocked_words`, `blocked_email_domains`, `strikes`, `ban_hours`, `review_min_seconds`, `review_max_seconds`, and `duplicate_window_hours` control validation and abuse limits. Put the CAPTCHA secret in `secrets.yml`. |
| `server.contact` | `dpo.name`, `dpo.email`, and the `general`, `abuse`, `admin`, and `security` destination `email` values configure report routing. Each destination supports `webhooks.telegram`, `webhooks.discord`, `webhooks.slack`, and `webhooks.generic` URL fields. |
| `server.notifications.email` | `smtp.host`, `smtp.port`, `smtp.username`, `smtp.password`, `smtp.tls`, `from.name`, and `from.email` configure mail delivery. Passwords belong in `secrets.yml`; supported `SMTP_*` environment variables override the corresponding SMTP values. |
| `server.privacy.data` | `sold`, `stored_on_server`, and `sharing[]` (`condition`, `when`, `data`) describe data use and sharing. |
| `server.privacy.retention` | `period`, `export_available`, and `deletion_available` describe operational retention and visitor rights. |
| `server.privacy.consent` | `show_until_acknowledged`, `message`, `message_if_sold`, `policy.text`, `policy.url`, `buttons.decline`, `buttons.accept`, `position`, `show_preferences`, and `preferences_text` configure the consent banner and preferences controls. |
| `server.privacy.cookies` | `essential.enabled/description`, `preferences.enabled/description`, and `analytics.enabled/description/description_suffix_not_sold/description_suffix_sold` describe cookie categories. Essential cookies remain enabled. |
| `server.privacy.third_party.services[]` | `name`, `purpose`, `data_sent`, and `policy_url` document configured external services. |
| `server.branding` | `title`, `tagline`, `description`, `logo`, `favicon`, and `content` configure the site identity and optional localized page content. `content` supports `header`, `footer`, `about`, `privacy`, and `terms`, with `format` and `body`. |
| `server.seo` | `keywords`, `author`, `og_image`, `twitter_handle`, `title_separator`, `canonical`, `noindex`, and `meta_tags` configure page metadata. `opengraph.enabled/type/site_name` and `twitter.enabled/card/creator` control social cards. `verification` supports Google, Bing, Yandex, Baidu, Pinterest, Facebook, and `custom[]` name/property/content entries. `sitemap.max_urls/include_images` and `feed.max_items/title` control discovery output. |
| `web-ui` | `theme`, `logo`, `favicon`, and `notifications.enabled/announcements` configure browser appearance and operator announcements. |
| `web-robots` | `allow[]` and `deny[]` set crawler rules and sitemap inclusion. |
| `web-security` | `admin`, `cors`, `report_url`, `contact`, `policy`, `keyservers[]`, and `publish_pgp_key` configure security headers, CORS, reporting links, and optional public-key publication. |
| `web-llms` | `enabled`, `include_endpoints`, `include_schemas`, and `custom_sections[]` control `/llms.txt` output. |

Configuration values are validated at load time; invalid values use their documented defaults and are logged. Not every field is safe to expose to the environment: use the file merge order and secret handling above for credentials.

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
