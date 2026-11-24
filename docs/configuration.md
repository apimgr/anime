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

## Live configuration reload

The server watches `server.yml`, `secrets.yml`, `yaml.d/`, and `assets/` (including nested asset directories). Changes are coalesced, all configuration files are re-merged, and the new live settings become active after in-flight HTTP requests complete. An unparseable file keeps its last valid copy; deleting a file removes its values from the merged configuration. Operator assets under `{config_dir}/assets/` are served from `/assets/` with revalidation caching.

Branding, SEO, theme, notification delivery and preferences, rate and security rules, discovery output, feature toggles, and scheduler schedules apply live. `server.port`, `server.address`, `server.ssl`, `server.daemonize`, `server.pidfile`, `server.database`, and `server.tor` require a restart; changing them logs the setting names while keeping the current runtime values. `SIGHUP` is ignored because filesystem changes trigger reload automatically.

## Common settings

| Setting | Area file | Purpose |
|---|---|---|
| `server.port`, `server.address`, `server.fqdn`, `server.mode`, `server.baseurl` | `server.yml` | Listen endpoint, public identity, operating mode, and path prefix |
| `server.daemonize`, `server.pidfile` | `server.yml` | Manual-start background behavior and PID-file checking; service managers and application containers stay in the foreground, and PID files are skipped in containers |
| `server.database.driver`, `.url`, `.pool.*` | `server.yml` | SQLite or libSQL connection and pool |
| `server.cache.type`, `.url`, `.host`, `.port`, `.tls`, `.tls_skip_verify`, `.tls_ca`, `.tls_cert`, `.tls_key`, `.pool_size`, `.min_idle`, `.timeout`, `.prefix`, `.ttl` | `server.yml` | In-process cache or external Valkey/Redis; credentials are stored in `secrets.yml`. TLS certificate verification is opt-in; configure a CA or client certificate by a filename under `{config_dir}/ssl/` |
| `server.tokens.max_linked`, `.cookie_max_age`, `.idle_purge_days` | `yaml.d/security.yml` | Owner-token policy |
| `server.rate_limit.read`, `.write`, `.health`, `.strict`, `.auth_failures`, `.global_burst` | `yaml.d/security.yml` | Sliding-window limits for requests, sensitive actions, and failed authentication |
| `server.trusted_proxies.additional`, `.client_ip_header` | `server.yml` | Trusted proxy addresses/CIDRs/DNS names and the pinned or automatic client-IP header |
| `server.pages.contact.enabled`, `.captcha`, `.antispam.*` | `yaml.d/pages.yml` | Contact/report form and abuse controls |
| `server.contact.*` | `yaml.d/contact.yml` | DPO, general, abuse, admin, and security contacts |
| `server.notifications.email.smtp.*`, `.from.*`, `.reply_to`, `.events.*` | `yaml.d/notifications.yml` | SMTP connection, sender, reply address, and event routing |
| `server.notifications.disk_low.*` | `yaml.d/notifications.yml` | Scheduled disk usage alerts and per-volume repeat interval |
| `server.compliance.*` | `yaml.d/privacy.yml` | Optional compliance profiles; all are disabled by default |
| `data.security_dir`, `data.cve.*`, `data.trivy.*` | `server.yml` | Security dataset storage and NVD/Trivy update settings |
| `cli.binary_download.dir`, `.require_auth` | `yaml.d/maintenance.yml` | Operator-provisioned CLI release directory and optional bearer-token requirement for downloads |
| `server.security.reports.retention_days` | `yaml.d/security.yml` | Closed-report retention |
| `web.security.report_url`, `.contact`, `.policy`, `.keyservers`, `.publish_pgp_key` | `yaml.d/web.yml` | Security.txt, disclosure policy, and key publication |
| `web.robots.allow`, `.deny` | `yaml.d/web.yml` | Crawler policy and sitemap inclusion |
| `web.llms.enabled`, `.include_endpoints`, `.include_schemas`, `.custom_sections` | `yaml.d/web.yml` | LLM discovery output |
| `server.update.branch`, `.auto_install`, `.defer_days`, `.cli_min_version` | `yaml.d/maintenance.yml` | Release channel and update behavior |

### Trusted proxies

Private, loopback, link-local, and the listen address's `/24` are trusted automatically. Add public proxy addresses, CIDRs, or DNS names under `server.trusted_proxies.additional`; DNS entries refresh every five minutes. Forwarded host, scheme, prefix, and client-IP headers are honored only when the immediate TCP peer is trusted. `client_ip_header: auto` checks the supported forwarding headers; setting a single header pins resolution to that header. Client IP is resolved right to left through the proxy chain and is used by rate limiting, GeoIP, blocklists, and audit logging.

## Cache

The cache supports `none`, `memory` (default), `valkey`, and `redis`. For an
external service, configure either `server.cache.url` or the individual
connection fields. Redis and Valkey URLs select the backend from their scheme;
`rediss://` and `valkeys://` enable TLS. URL credentials and the separate
`password` field are moved to `secrets.yml`. Configure the connection pool,
timeout, prefix, TTL, and optional private CA/client certificate files under
`server.cache` in `server.yml`.

Rate-limit counters use `server.db` by default and move to Valkey or Redis only
when that external cache is configured. The in-process `memory` cache does not
store rate-limit counters; it is only the fallback while an external cache is
unavailable.

When an external cache becomes unavailable, request rate limits continue with
an in-process sliding-window fallback and the public health response marks the
cache `degraded`. The server logs one warning for the outage and does not fail
open the limiter.

```yaml
server:
  cache:
    type: valkey
    url: ${CACHE_URL}
    pool_size: 10
    min_idle: 2
    timeout: 5s
    prefix: "anime:"
    ttl: 1h
```

## GeoIP country policy

GeoIP is a fail-open request risk signal. The server downloads the ASN, country,
and IPv4/IPv6 city MMDB files from `sapics/ip-location-db` into
`{data_dir}/security/geoip` and verifies each upstream SHA-256 checksum before
replacing a database. Private and internal addresses are not looked up. Missing
data never blocks requests; country policies are only applied when the lookup
returns a country code. `allow_countries` takes precedence over
`deny_countries`, and `server.security.allowlist` bypasses the country decision.
Presets are operator-authored reusable lists and are never enforced by name.

```yaml
server:
  geoip:
    enabled: true
    deny_countries: []
    allow_countries: []
    presets: {}
```

## Internal metrics

Metrics are disabled from public discovery and every service route requires its
own bearer token from `secrets.yml`. The tokens are generated during config
initialization. Configure firewall or proxy access separately; setting
`allow_unauthenticated` is suitable only on an isolated monitoring network.

```yaml
server:
  metrics:
    enabled: true
    root:
      enabled: true
    auth:
      allow_unauthenticated: false
    include_system: true
    include_runtime: true
    loki:
      max_entries: 1000
      max_age: 1h
```

The Prometheus service is available at `/server/metrics` and
`/server/metrics/prometheus`; Grafana dashboard JSON is at
`/server/metrics/grafana`, and sanitized recent HTTP access streams are at
`/server/metrics/loki`. Equivalent API aliases are `/api/v1/server/metrics`,
`/api/metrics`, and `/metrics` when the root alias is enabled. Send the matching
service token only as `Authorization: Bearer <token>`.

## Backup and restore

`--maintenance backup [file]` creates a full configuration archive, including
the local database when present, with a
`manifest.json`, per-file SHA-256 values, and immediate extraction verification.
Only `server.db` is included from the data directory by default. Use
`--maintenance backup --include-data` to include other data files and
`--maintenance backup --include-ssl` to include the config SSL directory.
When `server.backup.encryption_password` exists in `secrets.yml`, the archive is
encrypted with AES-256-GCM using an Argon2id-derived key and receives an `.enc`
suffix. Backups do not prompt for a password. If any `server.compliance`
standard is enabled, an unset password blocks backup creation.

`--maintenance restore <file>` stages and verifies the archive before copying
files into configured directories. It rejects path traversal, symbolic links,
special files, excessive archive expansion, and corrupt SQLite databases.
Encrypted restores prompt for the backup password; existing installations
also require root confirmation or the operator token. Legacy unencrypted
tar.gz archives are accepted through the same bounded extractor.

```yaml
server:
  backup:
    encryption:
      enabled: false
    retention:
      max_backups: 1
      keep_weekly: 0
      keep_monthly: 0
      keep_yearly: 0
      max_total_size: "10%"
  restore:
    max_archive_size: "1G"
    max_expanded_size: "4G"
    max_files: 10000
    max_file_size: "512M"
    max_ratio: 100
  compliance:
    gdpr: false
    ccpa: false
    hipaa: false
    soc2: false
```
| `server.scheduler.timezone`, `.catch_up_window`, `.tasks` | `yaml.d/maintenance.yml` | Built-in task schedules and retry settings |
| `server.backup.*`, `server.restore.*` | `yaml.d/maintenance.yml` | Backup retention, encryption, and restore limits |
| `server.geoip.*` | `yaml.d/security.yml` | GeoIP enablement and operator-authored country policies |
| `server.metrics.*` | `yaml.d/metrics.yml` | Authenticated Prometheus, Grafana, and Loki endpoints |

## Complete server settings index

The following list covers the server configuration fields. A field's YAML area is shown in the preceding file table; legacy values are accepted during migration from `server.yml` and then written to their assigned area. Secret values are loaded from `secrets.yml` last.

| YAML key or key group | Fields and behavior |
|---|---|
| `server` | `port`, `address`, `fqdn`, `baseurl`, and `mode` set the listener and public identity. `update.branch`, `update.auto_install`, `update.defer_days`, and `update.cli_min_version` control updates; legacy `server.update_branch` is migrated to `server.update.branch`. `scheduler.timezone`, `scheduler.catch_up_window`, and `scheduler.tasks.*` describe scheduled task policy. `metrics.enabled`, `metrics.root.enabled`, `metrics.auth.allow_unauthenticated`, `metrics.auth.tokens.*`, `metrics.include_system`, `metrics.include_app`, `metrics.include_runtime`, `metrics.duration_buckets`, `metrics.size_buckets`, and `metrics.loki.*` configure authenticated monitoring and histogram buckets. Metrics service tokens are generated into `secrets.yml`. `logging.access_format` and `logging.level` set access-log format and verbosity; per-log `logs.debug` is configurable like other log policies. |
| `server.database` | `driver`, `url`, and `pool.max_open`, `pool.max_idle`, `pool.max_lifetime`, `pool.max_idle_time` select SQLite/libSQL and connection-pool limits. |
| `server.i18n` | `default_language` and `available_languages` define the server locale fallback and enabled language set. The bundled catalogs currently support `en`, `es`, `fr`, `de`, `it`, and `ar`. |
| `server.rate_limit` | `read`, `write`, `health`, `strict`, and `auth_failures` each configure `requests` and `window`; `global_burst` sets the per-minute ceiling across endpoint classes. Anonymous requests use the resolved client IP, valid owner and operator tokens use a token-specific key, and Tor requests use the PROXY-exported circuit ID or the `tor` sentinel. Counters are sliding-window values. |
| `server.tokens` | `max_linked`, `cookie_max_age`, and `idle_purge_days` cap linked tokens, set the owner-cookie lifetime, and configure idle-owner cleanup. |
| `server.security` | `encryption_key` and `encryption_key_version` protect encrypted report data and key rotation. `reports.retention_days` controls closed security-report retention. Put the encryption key in `secrets.yml`. |
| `server.security.allowlist` | IP addresses and CIDR ranges that bypass the GeoIP country decision. |
| `server.geoip` | `enabled`, `deny_countries`, `allow_countries`, and named `presets` configure the fail-open country risk signal. Empty country lists allow all countries. `allow_countries` takes precedence when both lists are set. |
| `server.metrics` | `enabled`, `root.enabled`, `auth.allow_unauthenticated`, `include_system`, `include_runtime`, and `loki.max_entries/max_age` configure internal metrics. The `prometheus`, `grafana`, and `loki` bearer tokens are generated and stored in `secrets.yml`. |
| `server.scheduler` | `timezone`, `catch_up_window`, and `tasks.<name>.schedule/enabled/retry_on_fail/retry_delay` configure the in-process scheduler. |
| `server.maintenance.cleanup` | `disk_threshold` controls when disk-pressure cleanup begins; the default is 90 percent used. |
| `server.tor` | Tor binary and hidden-service resource, bootstrap, network, and bandwidth settings. Tor starts automatically when a Tor binary is available; there is no enable toggle. |
| `server.i2p` | I2P provider and eepsite settings. I2P is opt-in outside containers and defaults on in a container. `I2P_ENABLED` and `--i2p` override the configured setting; `--i2p=false` disables it. |
| `server.ssl.letsencrypt` | `enabled`, `email`, `challenge`, `staging`, `dns_provider`, and `dns_credentials` configure certificate issuance. DNS provider credentials belong in `secrets.yml`; staging uses the certificate authority's staging service. |
| `server.pages.contact` | `enabled`, `captcha`, `captcha_site_key`, `captcha_secret_key`, and `success_message` control the public report form and CAPTCHA. `antispam.honeypot`, `min_fill_seconds`, `max_age_seconds`, `per_ip`, `per_email`, `global`, `window`, `max_links`, `reject_markup`, `blocked_words`, `blocked_email_domains`, `strikes`, `ban_hours`, `review_min_seconds`, `review_max_seconds`, and `duplicate_window_hours` control validation and abuse limits. Put the CAPTCHA secret in `secrets.yml`. |
| `server.contact` | `dpo.name`, `dpo.email`, and the `general`, `abuse`, `admin`, and `security` destination `email` values configure report routing. Each destination supports `webhooks.telegram`, `webhooks.discord`, `webhooks.slack`, and `webhooks.generic` URL fields. |
| `server.notifications.email` | `smtp.host`, `smtp.port`, `smtp.username`, `smtp.password`, `smtp.tls`, `from.name`, `from.email`, `reply_to`, and `events.*` configure mail delivery and event routing. `reply_to` is also the default recipient for the email test command. Passwords belong in `secrets.yml`; supported `SMTP_*` environment variables override the corresponding SMTP values. |
| `server.notifications.disk_low` | `enabled`, `threshold_percent`, `check_interval`, and `repeat` configure scheduled low-disk checks and per-volume notification deduplication. Defaults are 85 percent, five minutes, and 24 hours. |
| `server.cache` | `type`, connection fields, `tls`, `tls_skip_verify`, `tls_ca`, `tls_cert`, `tls_key`, pool settings, `timeout`, `prefix`, and `ttl` configure the cache. TLS verification is selected with `tls_skip_verify: false`; CA and client-certificate filenames are resolved under `{config_dir}/ssl/`. |
| `server.compliance` | `gdpr`, `ccpa`, `hipaa`, `soc2`, `pci_dss`, `iso27001`, `fedramp`, `lgpd`, `pipeda`, `appi`, and `pdpa` are optional compliance profiles, all disabled by default. Enabling any profile activates the stricter backup encryption requirements. |
| `server.notifications.disk_low` | `enabled`, `threshold_percent`, `check_interval`, and `repeat` configure scheduled low-disk checks and per-volume notification deduplication. Defaults are 85 percent, five minutes, and 24 hours. |
| `data` | `security_dir` sets the base directory for security datasets; `cve.source`, `cve.filter_by_cpe`, and `cve.api_key` configure NVD updates; `trivy.enabled` and `trivy.source` configure the optional Trivy dataset. CPE filtering defaults off so the full NVD feed is retained. The API key belongs in `secrets.yml`. |
| `cli.binary_download` | `dir` selects the operator-provisioned release assets directory; `require_auth` defaults to false and can require a bearer token for downloads. The server does not fetch or build those assets. |
| `server.privacy.data` | `sold`, `stored_on_server`, and `sharing[]` (`condition`, `when`, `data`) describe data use and sharing. |
| `server.privacy.retention` | `period`, `export_available`, and `deletion_available` describe operational retention and visitor rights. |
| `server.privacy.consent` | `show_until_acknowledged`, `message`, `message_if_sold`, `policy.text`, `policy.url`, `buttons.decline`, `buttons.accept`, `position`, `show_preferences`, and `preferences_text` configure the consent banner and preferences controls. |
| `server.privacy.cookies` | `essential.enabled/description`, `preferences.enabled/description`, and `analytics.enabled/description/description_suffix_not_sold/description_suffix_sold` describe cookie categories. Essential cookies remain enabled. |
| `server.privacy.third_party.services[]` | `name`, `purpose`, `data_sent`, and `policy_url` document configured external services. |
| `server.branding` | `title`, `tagline`, `description`, `logo`, `favicon`, and `content` configure the site identity and optional localized page content. `content` supports `header`, `footer`, `about`, `privacy`, and `terms`, with `format` and `body`. |
| `server.seo` | `keywords`, `author`, `og_image`, `twitter_handle`, `title_separator`, `canonical`, `noindex`, and `meta_tags` configure page metadata. `opengraph.enabled/type/site_name` and `twitter.enabled/card/creator` control social cards. `verification.google`, `.bing`, `.yandex`, `.baidu`, `.pinterest`, `.facebook`, and `custom[]` name/property/content entries configure verification metadata. `sitemap.max_urls/include_images` and `feed.max_items/title` control discovery output. |
| `web-ui` | `theme`, `logo`, `favicon`, and `notifications.enabled/announcements` configure browser appearance and operator announcements. |
| `web-robots` | `allow[]` and `deny[]` set crawler rules and sitemap inclusion. |
| `web-security` | `admin`, `cors`, `report_url`, `contact`, `policy`, `keyservers[]`, and `publish_pgp_key` configure security headers, CORS, reporting links, and optional public-key publication. |
| `web-llms` | `enabled`, `include_endpoints`, `include_schemas`, and `custom_sections[]` control `/llms.txt` output. |

Configuration values are validated at load time; invalid values use their documented defaults and are logged. Not every field is safe to expose to the environment: use the file merge order and secret handling above for credentials.

## Scheduler

The scheduler always runs in-process and stores task state and execution history
in `server.db`. Calendar schedules use the IANA timezone configured by
`server.scheduler.timezone`; supported schedules are five-field cron,
`@hourly`, `@daily`, `@weekly`, `@monthly`, and `@every` durations. Operators
can inspect and manage persisted tasks with
`--maintenance scheduler list|show|run|enable|disable|history`.

Built-in tasks include SSL renewal, GeoIP and security-dataset refresh, keyserver
retry, token and security cleanup, log rotation, daily and optional hourly
backups, disk checks, health checks, and daily update checks. Tor and I2P health
tasks are registered only when their overlay is active. Task configuration and
run history persist in SQLite; `run` queues execution for the live scheduler or
the next startup. A failed task is audited and can be retried according to its
`retry_on_fail` and `retry_delay` settings.

## Email notifications

SMTP settings are under `server.notifications.email.smtp`; the password belongs
in `secrets.yml`. Mail delivery is enabled only when SMTP is configured and a
valid admin recipient is present at `server.contact.admin.email`. Startup probes
SMTP and reports its status through the server's email health fields. Templates
are customizable under the configuration template directory, use translation
keys, and fall back to bundled localized defaults. `events` maps event names to
email preferences; operator events are logged even when their email preference
is off. Events cover backup completion/failure, SSL expiry and renewal, staged,
installed and failed updates, security alerts, database and scheduler errors,
low disk space, and configured startup/shutdown notifications.

## Scheduled backups

`backup_daily` runs at 02:00 and creates a verified full snapshot plus a daily
incremental. `backup_hourly` is disabled by default and, when enabled, replaces
one hourly incremental based on the latest daily full. Retention protects every
full snapshot still needed by an incremental chain. A run is skipped with an
ERROR-level `backup.skipped_disk_full` audit event if the disk threshold is
exceeded or free space is below twice the newest backup's size. A skip is
recorded as skipped and does not send a backup-failed email.

## Update policy

`server.update.defer_days` gates scheduled checks for stable and beta releases;
it does not delay manual `--update check` or `--update yes`, and is ignored for
the rolling devel channel. `auto_install` defaults to false. When enabled, a
root-started Unix server stages checksum-verified binaries for the next
privileged restart; a normal user service or Windows service stays notify-only.
See [Installation](installation.md#system-service) for supervisor restart
behavior and service privilege handling.

## Tor and I2P overlays

The server owns separate loopback HTTP listeners for Tor and I2P. Tor starts
when its binary is available and otherwise remains disabled. I2P uses a
dedicated `i2pd` process when installed, or a loopback SAM bridge when enabled.
The Tor listener consumes PROXY protocol; the I2P listener is plain HTTP. Their
health and published addresses are shown on `/server/healthz` and the server
status page. Configure I2P at `server.i2p`; Tor and I2P data/config paths are
derived from the application directories.

## Branding and page content

`yaml.d/branding.yml` holds `server.branding`, `server.seo`, and `web.ui`. Branding fields include the site title, tagline, description, logo, and favicon. SEO settings control canonical URLs, metadata, social cards, verification tags, sitemap limits, and feed titles and item counts.

Operator-authored page content is optional and uses `server.branding.content`:

```yaml
server:
  branding:
    content:
      header:
        format: html
        body: '<span>Operator notice</span>'
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

`header` and `footer` accept sanitized HTML only and appear on every server-rendered page, including preferences and error pages. `about`, `privacy`, and `terms` accept `md`, `html`, or `txt`; Markdown and HTML are sanitized before rendering. A localized `body` map selects the current language, then `default`; a plain string applies to every language. Each body is limited to 256 KiB. Operator content cannot link to `/server/contact`. About, privacy, and terms content extends the built-in page, except terms content replaces the built-in terms. The footer entry replaces only its branding block; the shared application footer remains.

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
## Security data updates

Runtime security datasets live under `{data_dir}/security/`: blocklists, the
NVD CVE index, the optional Trivy vulnerability database, and GeoIP databases.
The scheduler refreshes blocklists and NVD daily. The built-in FireHOL IP list
is always active; `server.security.blocklists.extra_sources` adds HTTPS IP or
domain lists. Failed or invalid updates keep the last good directory active,
and health reports missing or stale data as `error` or `degraded`.

The NVD endpoint and optional OCI Trivy reference are configured under
`data.cve.source` and `data.trivy`. Set `data.trivy.enabled: true` to enable
Trivy downloads. Store the optional NVD API key under `data.cve.api_key` in
`secrets.yml`; it is removed from the ordinary YAML configuration during
secret migration. NVD requests are paged and rate limited; HTTP 403/429 errors
are retried by the next scheduled run without preventing startup.

## TLS and log rotation

TLS settings live under `server.ssl`. Certificate discovery checks the system
Let's Encrypt paths, then `{config_dir}/ssl/letsencrypt/{fqdn}/`, then
`{config_dir}/ssl/local/{fqdn}/`; certificates must be readable, unexpired,
and valid for the configured FQDN. `server.ssl.letsencrypt.enabled` enables
automated issuance and daily renewal seven days before expiry. HTTP-01 is
served by the application HTTP listener, TLS-ALPN-01 by its HTTPS listener,
and DNS-01 uses a lego provider named by `dns_provider`. Put provider
environment-variable credentials in `server.ssl.letsencrypt.dns_credentials`;
they are encrypted at rest in `secrets.yml` using the installation secret.
Set `server.port` to one port for HTTP (or HTTPS when SSL is enabled), or to
`"<http-port>,<https-port>"` for simultaneous HTTP and HTTPS. In dual-port
mode, SSL must be enabled and the second configured port serves HTTPS, even
when it is a custom port such as `8443`.
Bind ports 80 and 443 (or forward those ports to the application) for HTTP-01
and TLS-ALPN-01; use DNS-01 where the listener cannot receive those challenges.

`server.logs` configures per-log `filename`, `rotate`, `keep`, `format`, and
`compress` policies. The daily `log_rotation` scheduler task applies age and
size limits, then applies each log's retention policy. Defaults are weekly or
50 MB, with no retained rotated files; access logs rotate monthly and audit
logs daily. The task operates on regular files in the configured OS log
directory and rejects path components in configured filenames.
