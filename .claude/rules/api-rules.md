# API Rules (PART 13, 14, 15)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never expose sensitive internals in health, API, or error responses.
- Never publish undocumented routes or keep project-local legacy aliases.
- Never hardcode hostnames, ports, or schemes into generated URLs.
- Never serve an authenticated or private response with public caching.

## CRITICAL - ALWAYS DO

- Provide the versioned REST API, synchronized OpenAPI/Swagger, and GraphQL surfaces.
- Keep every route in the documented public discovery/API inventory and remove obsolete project-local aliases.
- Negotiate HTML, JSON, and text formats as specified and use canonical envelopes.
- Provide complete dynamic health checks and stable version/build metadata.
- Build absolute URLs from trusted request context and configured URL values.
- Support TLS 1.2+, manual certificates, and configured ACME behavior.

## Key rules

- Health is available at `/server/healthz` and `/api/{api_version}/server/healthz`.
- Swagger and GraphQL UI/API routes must be complete and documented.
- Root aliases are opt-in only where the relevant config defines them.
- Public health/discovery surfaces must not disclose credentials, private paths, internal IPs, or protected owner data.

For complete details, see AI.md PART 13, 14, 15
