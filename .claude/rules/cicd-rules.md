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
- Keep equivalent GitHub, Gitea, Forgejo, GitLab, and Jenkins pipelines, including beta/devel/release and Docker workflows.
- Go jobs use the maintained `casjaysdev/go:latest` image directly; do not add `Dockerfile.build`, build-image preflight, or toolchain-image workflows.
- Verify the real provider run after each push; a pending or failed run leaves the work incomplete.
- Expand every generated release name, description, note, and asset path before publishing; never publish a literal workflow expression or placeholder.
- CI versions are derived from the event/ref/commit and never from `release.txt`; Docker images build on every branch push as specified.
- Keep workflow permissions least-privilege and releases verifiable.

For complete details, see AI.md PART 27
