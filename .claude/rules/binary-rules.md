# Binary Rules (PART 7, 8, 32)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never enable CGO or require an external runtime dependency.
- Never hand-roll server flag parsing or hardcode a renamed binary's display name.
- Never let development mode alone expose pprof or debug endpoints.
- Never make the client require a GUI or TUI for command-line workflows.

## CRITICAL - ALWAYS DO

- Embed application assets and build static binaries for all 8 OS/architecture pairs.
- Support `--help`, `--version`, `--shell`, `--debug`, `--color`, and `--lang`.
- Detect TTY, display, SSH, and `TERM=dumb`; respect `NO_COLOR`.
- Detect stale PID files and skip PID files in containers.
- Ship the required `anime-cli` client with server discovery, configuration, and updates.

## Key rules

- Show the actual executable name in help, version, and user-facing errors.
- Keep server startup in the binary; no setup script is required.
- The client supports CLI, TUI, and GUI modes while preserving headless errors.

For complete details, see AI.md PART 7, 8, 32
