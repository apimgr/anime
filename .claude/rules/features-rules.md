# Feature Rules (PART 17, 18, 19, 20, 21, 22)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never use cron, Task Scheduler, or another external scheduler.
- Never expose metrics publicly without the required per-service bearer token.
- Never treat unavailable GeoIP data as a reason to block normal service startup.
- Never restore untrusted archives directly into live filesystem paths.
- Never auto-install an update that fails checksum or minimum-version policy.

## CRITICAL - ALWAYS DO

- Configure SMTP, queue delivery with retry, and log success/failure safely.
- Run scheduled tasks in-process and expose their status and manual triggers.
- Update GeoIP from the specified sources and support country rules.
- Keep metrics internal and provide Prometheus, Grafana, and Loki endpoints.
- Encrypt and verify backups; make restore atomic and enforce retention.
- Verify update checksums, stage replacements atomically, and support rollback.

## Key rules

- SMTP is optional and must degrade gracefully when unavailable.
- Compliance mode requires encrypted backups and a password.
- Never log credentials or expose raw secrets in API responses.

For complete details, see AI.md PART 17, 18, 19, 20, 21, 22
