# Remaining anime implementation work

The IDEA.md product surfaces and prior CI-failure fixes are implemented. These shared AI.md requirements remain incomplete and must not be treated as completed:

## Companion CLI

- Implement the client update contract (autodiscover metadata, platform binary download, checksum verification, atomic replacement, and minimum-version refusal). The current project API does not publish client binaries or an autodiscover response.
- Complete the cli.yml configuration and display-mode behaviors, including config-selected output/theme and profile selection. The current CLI persists its server URL but does not expose all shared config keys or profile handling.
- Expand shell completions to every required shell and ensure the scripts accurately complete flags and commands.

## Server operations

- Replace the existing update and maintenance placeholder messages with secure, complete behavior, or remove unsupported commands and their documentation until implementation exists.
- Finish service lifecycle support across Linux, macOS, Windows, BSD, systemd, runit, rc.d, and launchd. The current implementation shells out only for Linux/macOS and contains fixed paths/identifiers.
- Complete runtime privilege separation, service account creation, signal-driven config reload, and the operator-only server configuration requirements from AI.md.

## Shared platform features

- Implement the remaining mandatory AI.md server subsystems that this read-only quote product does not currently provide: I2P, internal metrics, scheduler, GeoIP/blocklists, notification delivery, backup/restore, and associated server APIs/CLI flows.
- Replace the current route-reference page with the full interactive Swagger UI and finish the OpenAPI contract, including response/error schemas and every shipped API route.
- Complete the shared PWA, accessibility, internationalization, and browser test requirements. The current frontend has existing inline handlers/scripts and does not provide the full shared theme, language, and no-JavaScript behavior.
- Bring container startup into full AI.md conformance. The current entrypoint supervises child processes in Bash and does not `exec` the server process; the server also does not yet own all Tor and privilege setup described by the shared spec.

## CI and release

- Extend CI to enforce build/test/lint/coverage gates for both binaries and validate the docs and Docker Compose contracts. Release workflows now produce the client binary, but remote Actions have not run for these unpushed changes.
- Set the ReadTheDocs `site_url` after the canonical hosted documentation URL is confirmed; no authoritative URL is present in IDEA.md or repository metadata.
