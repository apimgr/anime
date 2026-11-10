# AI.md alignment work

This task list was rebuilt from `AI.md` because the previous task inventory was not reliable. Completion requires implementation, documentation alignment, required verification, and the prescribed commit workflow. AI.md PART 33 is reference material and remains read-only.

## Foundation and binary

- [ ] Reconcile repository structure, license/dependency notices, AI loader/rule files, memory, and IDEA.md with PARTS 0–4 and PART 33.
- [ ] Complete configuration schema, path validation, CLI/environment/file precedence, first-run behavior, and mode handling from PARTS 5, 6, 8, and 12.
- [ ] Complete binary startup, embedded assets, cross-platform naming/build behavior, command handling, daemon/status/service flags, PID management, and signal behavior from PARTS 7–9.
- [ ] Complete canonical errors, debug-only diagnostics, cache validators and version-change purge behavior from PARTS 6, 9, and 11.

## Server data, API, and security

- [ ] Implement the SQLite/libsql data layer, full idempotent schema, query timeouts, and startup/recovery behavior from PARTS 5, 10, and 11.
- [ ] Implement security and operational logging, audit events, sanitization, maintenance state, request IDs, path checks, rate limits, and public endpoint safety from PARTS 11–12.
- [ ] Complete health/version endpoints and all server/API routes, canonical response envelopes, content negotiation, OpenAPI, GraphQL, CORS/CSRF/CSP, and route tests from PARTS 13–14.
- [ ] Complete TLS certificate lifecycle and ACME/Let's Encrypt behavior from PART 15.
- [ ] Complete project frontend requirements: SSR, progressive enhancement, all pages/forms, themes, PWA, accessibility, security headers, and browser behavior from PART 16.

## Required platform features

- [ ] Implement notifications/email and their configuration, templates, delivery, and test behavior from PART 17.
- [ ] Complete the built-in scheduler, persistence, lifecycle, and task history from PART 18.
- [ ] Implement GeoIP and country/IP/domain allow/block controls from PART 19.
- [ ] Implement internal operational metrics and the specified public/debug surfaces from PART 20.
- [ ] Replace unsafe/incomplete backup handling with verified, atomic, encrypted backup and restore flows from PART 21.
- [ ] Complete secure update checks/installations and rollback behavior from PART 22.
- [ ] Implement privilege escalation and safe service installation/removal from PART 23.
- [ ] Implement all required service managers and platform behavior from PART 24.

## Build, delivery, and client

- [ ] Reconcile the Makefile to the exact supported target set and Docker-only toolchain rules in PART 25.
- [ ] Complete Docker build, runtime, compose, networking, volume, and process behavior from PART 26.
- [ ] Reconcile all CI/CD providers, pinned actions, required jobs, release artifacts, and security checks with PART 27 and provider conventions.
- [ ] Add all four required test scripts, unit/integration coverage, endpoint assertions, browser E2E tiers, and supported container/Incus execution from PART 28.
- [ ] Complete MkDocs/ReadTheDocs configuration and documentation pages from PART 29.
- [ ] Complete server and client internationalization plus accessibility requirements from PART 30.
- [ ] Implement the required app-scoped Tor service and optional opt-in I2P support from PART 31.
- [ ] Complete the CLI client, config/profiles, output modes, shell completions, commands, update contract, and tests from PART 32.

## Final reconciliation and release gate

- [ ] Reconcile IDEA.md with actual product behavior and PART 33; synchronize README, docs, OpenAPI, GraphQL, and help output.
- [ ] Review every changed file and remove obsolete code, routes, configuration, and documentation that conflict with AI.md.
- [ ] Run the prescribed build, test, lint, coverage, security, and integration gates in the primary session and resolve all failures.
- [ ] Complete `.git/COMMIT_MESS`, run `gitcommit --dir` as the only commit/push path, then inspect and resolve the resulting GitHub CI run.
- [ ] service.go prints unconditional emoji status output (src/service/service.go:178,179,208,249,267) — gate on NO_COLOR/--color (found by go-lint, pre-existing)
