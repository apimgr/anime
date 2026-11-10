# Makefile Rules (PART 25)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never use Makefile targets in CI/CD workflows.
- Never compile, test, or lint Go directly on the host.
- Never write test/build runtime data into the source tree.
- Never use unbounded cleanup or remove artifacts outside this project.

## CRITICAL - ALWAYS DO

- Provide the required local build, release, Docker, test, dev, clean, and help targets.
- Run Go tooling in `casjaysdev/go:latest` with CGO disabled and resource limits.
- Use project-scoped Go build caches and `binaries/` / `releases/` outputs.
- Keep Docker builds local by default; only explicit release workflows publish.

## Key rules

- Infer project name and organization from the Git remote, with path fallback.
- Make test includes vet, vulnerability scanning, coverage, and test execution.
- Keep output and temporary artifacts in the locations prescribed by AI.md.

For complete details, see AI.md PART 25
