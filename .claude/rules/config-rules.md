# Configuration Rules (PART 5, 6, 12)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Never use runtime config mutation APIs or an admin settings UI.
- Never ignore unknown configuration keys or invalid boolean values.
- Never enable debug endpoints from development mode alone.
- Never weaken security protections as a convenience default.

## CRITICAL - ALWAYS DO

- Use `server.yml` as the only source of operator configuration.
- Keep secrets in `secrets.yml`; load ordered config drop-ins from `yaml.d/` only where the spec defines them.
- Apply precedence: CLI flags, environment, file, defaults.
- Parse booleans through `config.ParseBool` or `config.IsTruthy`.
- Validate paths, ports, hosts, and every loaded configuration value.
- Hot-reload safe settings and report settings that require restart.
- Reject unknown keys and invalid values; preserve file-only configuration with no settings API or admin UI.

## Key rules

- Defaults must work without a config file; the first run writes `server.yml`.
- Use a random unused 64000–64999 port on first run and persist it.
- `--debug`/`DEBUG` controls debug endpoints; `MODE=debug` defaults debug on.
- Never store rate-limit counters in the general-purpose cache; follow PART 12's explicitly named persistent/default backend.

For complete details, see AI.md PART 5, 6, 12
