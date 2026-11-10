# CI/CD Rules (PART 27)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never use Makefile targets in CI or install toolchains inline in workflow steps.
- Never use unpinned third-party actions or Dependabot.
- Never expose secrets to forked pull requests or print credentials in logs.
- Never claim a push is complete until the real provider CI run passes.

## CRITICAL - ALWAYS DO

- Match the workflow provider to the Git remote and follow its native conventions.
- Use `casjaysdev/go:latest` for Go build/test/lint jobs with least privilege.
- Run secret scanning, vulnerability scanning, license checks, and coverage gates.
- Keep build/release workflows across all supported target platforms.
- Inspect the triggered CI run after every push and fix failures before completion.

## Key rules

- Go projects do not use a custom build image or toolchain preflight workflow.
- Pin third-party actions to verified full commit SHAs with Renovate annotations.
- Keep workflow permissions least-privilege and releases verifiable.

For complete details, see AI.md PART 27
