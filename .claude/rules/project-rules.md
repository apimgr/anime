# Project Rules (PART 2, 3, 4)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never change the MIT project license or use GPL, AGPL, LGPL, or CGO dependencies.
- Never place Go source outside `src/` or create `pkg/`, `internal/`, or `cmd/`.
- Never use container-only paths for native installations.
- Never change frozen on-disk identifiers without an explicit rename migration.
- Never treat IDEA.md as implementation authority when it conflicts with AI.md; correct IDEA.md instead.

## CRITICAL - ALWAYS DO

- Attribute every embedded dependency in `LICENSE.md` and keep it current.
- Support Linux, macOS, Windows, and BSD on amd64 and arm64.
- Resolve project identity from the Git remote and paths from the platform and run context.
- Set `internal_name` and `internal_org` from project identity on first setup and keep both stable across ordinary renames.
- Use `server.yml`, with OS-specific config, data, cache, log, backup, and PID paths.

## Key rules

- Keep templates and static assets embedded in the single binary.
- Keep runtime data outside the repository and separate config, data, cache, and logs.
- Keep the root file set and `src/` package layout within AI.md's exhaustive allowlists; `docs/` contains only the ReadTheDocs site.
- Keep source, docs, and project tracking inside the repo; put runtime/test output in `/tmp/{project_org}/{internal_name}-XXXXXX`.
- Docker uses `/config` and `/data`; native platforms use PART 4 paths.

For complete details, see AI.md PART 2, 3, 4
