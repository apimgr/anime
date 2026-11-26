# Backend Rules (PART 9, 10, 11, 31)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never return stack traces, secrets, internal paths, or credentials to clients or logs.
- Never use `SELECT *`, string-built SQL, unsafe archive extraction, or CGO drivers.
- Never log tokens or passwords; never serve active user content on the app origin.
- Never make Tor or I2P a required runtime dependency for ordinary service availability.
- Never let general log rotation rewrite the append-only, hash-chained audit log.

## CRITICAL - ALWAYS DO

- Use stable JSON error codes and appropriate HTTP status codes.
- Use SQLite by default, idempotent schema setup, bounded queries, and connection pooling.
- Apply security headers, request limits, rate limits, and JSON audit/security logging.
- Preserve audit-chain integrity across rotation, retention, compression, and filename reload; use canonical audit actor types and never log credentials.
- Keep raw log files free of ANSI color and emoji; use the required access, auth, error, and app log formats.
- Start Tor under the service identity when available; keep I2P opt-in and fail gracefully.
- Version and revalidate cached assets; prevent stale service-worker caches after updates.

## Key rules

- Schema changes are additive and idempotent; no migration files or version table.
- Use Argon2id where AI.md requires password hashing; use the specified authenticated encryption for recoverable secrets and protected report data.
- Security checks remain active in every application mode.
- Overlay networking never blocks the public quote service from starting.
- Tor circuit identity uses the protocol-defined circuit ID when available and never substitutes an IP address; I2P is opt-in outside containers and enabled by container defaults.

For complete details, see AI.md PART 9, 10, 11, 31
