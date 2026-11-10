# Project Rules (PART 2, 3, 4)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never change the MIT project license or use GPL, AGPL, LGPL, or CGO dependencies.
- Never place Go source outside `src/` or create `pkg/`, `internal/`, or `cmd/`.
- Never use container-only paths for native installations.
- Never change frozen on-disk identifiers without an explicit rename migration.

## CRITICAL - ALWAYS DO

- Attribute every embedded dependency in `LICENSE.md` and keep it current.
- Support Linux, macOS, Windows, and BSD on amd64 and arm64.
- Resolve project identity from the Git remote and paths from the platform and run context.
- Use `server.yml`, with OS-specific config, data, cache, log, backup, and PID paths.

## Key rules

- Keep templates and static assets embedded in the single binary.
- Keep runtime data outside the repository and separate config, data, cache, and logs.
- Docker uses `/config` and `/data`; native platforms use PART 4 paths.

For complete details, see AI.md PART 2, 3, 4
