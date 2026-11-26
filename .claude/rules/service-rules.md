# Service Rules (PART 23, 24)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never run the production service permanently as root without an IDEA.md exception.
- Never shell out with untrusted input or use fixed service paths/identifiers.
- Never report a platform's service operation as supported when it is not implemented.

## CRITICAL - ALWAYS DO

- Create and use the dedicated service identity where the platform supports privilege drop.
- Complete privileged filesystem and socket setup before dropping privileges.
- Follow PART 23's ordered startup: inspect only approved bootstrap settings as root, prepare/bind protected resources, drop and verify identity, then load full configuration and start the runtime.
- Support every platform manager specified by PART 24: systemd, OpenRC, SysVinit, runit, s6, launchd, BSD rc.d, and Windows Service Control Manager.
- Require authorization for destructive or operator-only maintenance actions.

## Key rules

- Check capabilities and permissions instead of assuming escalation access.
- Resolve service paths and names from stable project variables.
- Preserve foreground behavior under modern service managers and containers.
- PID handling obeys the configured `server.pidfile` enable/disable state and uses the canonical resolved path.

For complete details, see AI.md PART 23, 24
