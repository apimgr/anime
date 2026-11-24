# AI Assistant Rules (PART 0, 1)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never edit `AI.md`; it is the read-only implementation source of truth.
- Never guess when a requirement is unclear or conflicts with another rule.
- Never add features or preserve obsolete project behavior contrary to the spec.
- Never claim work is complete without checking the relevant behavior.
- Never expose secrets, internal paths, or stack traces to clients.

## CRITICAL - ALWAYS DO

- Apply the hierarchy `SPEC.md > AI.md > global instructions`; IDEA.md describes WHAT and never overrides implementation rules.
- Read the complete relevant PART and its cross-references before implementation.
- Keep `IDEA.md`, README, docs, OpenAPI, and GraphQL aligned with the shipped product.
- Validate untrusted input, use canonical API envelopes, and keep server configuration file-only.
- Treat internet-facing traffic as hostile and preserve authentication and abuse controls.
- After compaction, reread only the relevant AI.md sections; return to the prior point after following a cross-reference.
- Keep project task tracking in `TODO.AI.md`/`TODO.md`; shared `TODO.API.md` is only for verified defects inside AI.md.

## Key rules

- Follow the relevant sections of `AI.md` in dependency order and verify each subsystem.
- Implement every mandatory PART and the required companion client.
- Report unresolved spec ambiguity instead of silently choosing behavior.
- Use a temporary audit file only for an explicitly requested audit and remove it after resolution; do not leave report-only artifacts.

For complete details, see AI.md PART 0, 1
