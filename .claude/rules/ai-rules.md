# AI Assistant Rules (PART 0, 1)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never edit `AI.md`; it is the read-only implementation source of truth.
- Never guess when a requirement is unclear or conflicts with another rule.
- Never add features or preserve obsolete project behavior contrary to the spec.
- Never claim work is complete without checking the relevant behavior.
- Never expose secrets, internal paths, or stack traces to clients.

## CRITICAL - ALWAYS DO

- Read the complete relevant PART and its cross-references before implementation.
- Keep `IDEA.md`, README, docs, OpenAPI, and GraphQL aligned with the shipped product.
- Validate untrusted input, use canonical API envelopes, and keep server configuration file-only.
- Treat internet-facing traffic as hostile and preserve authentication and abuse controls.

## Key rules

- Follow the relevant sections of `AI.md` in dependency order and verify each subsystem.
- Implement every mandatory PART and the required companion client.
- Report unresolved spec ambiguity instead of silently choosing behavior.

For complete details, see AI.md PART 0, 1
