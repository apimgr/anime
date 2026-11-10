# Docker Rules (PART 26)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never copy or symlink host-built binaries into a production image.
- Never use a privileged image user where privilege drop is required.
- Never expose internal ports on the host or bind host ports to all interfaces.
- Never run Tor as an entrypoint-managed background child.

## CRITICAL - ALWAYS DO

- Use the prescribed multi-stage Dockerfile and embed assets in the binary.
- Use `tini → entrypoint.sh → app`, with the server owning managed child processes.
- Provide production, development, and isolated test Compose files under `docker/`.
- Bind exposed host ports to `172.17.0.1` in the configured 62000–64999 range.
- Use health checks, resource limits, OCI metadata, and persistent volumes correctly.

## Key rules

- The production container stays in the foreground and receives graceful signals.
- Runtime files live in `/config` and `/data`, never the source checkout.
- Do not create a custom Go build image; use `casjaysdev/go:latest`.

For complete details, see AI.md PART 26
