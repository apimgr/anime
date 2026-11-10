# Configuration Rules (PART 5, 6, 12)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never use runtime config mutation APIs or an admin settings UI.
- Never ignore unknown configuration keys or invalid boolean values.
- Never enable debug endpoints from development mode alone.
- Never weaken security protections as a convenience default.

## CRITICAL - ALWAYS DO

- Use `server.yml` as the only source of operator configuration.
- Apply precedence: CLI flags, environment, file, defaults.
- Parse booleans through `config.ParseBool` or `config.IsTruthy`.
- Validate paths, ports, hosts, and every loaded configuration value.
- Hot-reload safe settings and report settings that require restart.

## Key rules

- Defaults must work without a config file; the first run writes `server.yml`.
- Use a random unused 64000–64999 port on first run and persist it.
- `--debug`/`DEBUG` controls debug endpoints; `MODE=debug` defaults debug on.

For complete details, see AI.md PART 5, 6, 12
