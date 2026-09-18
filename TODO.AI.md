# TODO

## CI/CD

- `docker/Dockerfile.dev` is missing. AI.md's CI/CD Rules require the `:devel`
  image to be built by the `build-devel` job in `.github/workflows/docker.yml`
  (daily `0 4 * * *` schedule + non-tag push + `workflow_dispatch`). The job
  is present but guarded with `if: hashFiles('docker/Dockerfile.dev') != ''`
  so it no-ops until the file exists — add `docker/Dockerfile.dev` to
  activate it.
- AI.md's CI/CD Rules § LDFLAGS specifies `-X 'main.CommitID=...' -X
  'main.BuildEpoch=...' -X 'main.OfficialSite=...'`, but `src/main.go` only
  declares `Version`, `Commit`, and `BuildDate` package vars (matches the
  existing `Makefile`). The new workflows (`ci.yml`, `daily.yml`,
  `release.yml`, `beta.yml`) were built against the vars that actually exist
  in the code (`main.Version` / `main.Commit` / `main.BuildDate`), not the
  spec's names, since `-X` against a nonexistent var silently embeds nothing.
  Reconcile AI.md and `main.go` — either rename the spec's LDFLAGS vars or
  add `CommitID`/`BuildEpoch`/`OfficialSite` to `main.go`.
- The project has zero `_test.go` files across all 10 `src/` packages.
  `ci.yml`'s `test` job enforces a 60% coverage minimum per AI.md's
  org-wide Go coverage rule; with no tests, coverage is 0% and that job
  will fail on the first real CI run. Add unit tests (table-driven where
  applicable) for `src/anime`, `src/config`, `src/data`,
  `src/mode`, `src/paths`, `src/scheduler`, `src/server`, `src/service`,
  `src/ssl`, and the root `src` package to reach the 60% threshold —
  writing a real test suite for a 10-package service is out of scope for
  a CI/CD-workflow-addition task and needs a dedicated pass.

## Admin UI Removal

- `src/admin/` (handlers.go, auth.go — a bearer-token/session-based admin
  web login and dashboard) was removed: AI.md explicitly forbids an admin
  web UI, dashboard, settings pages, or web routes for server
  administration ("Auth is bearer-token only ... and applies to /api/
  routes, never to HTML pages"; "Server administration is via config file
  (no admin web panel)"). The wiring in `src/server/server.go`
  (`admin.NewHandler`, `RegisterRoutes`, the import) and the
  `ServerConfig.Admin` / `ServerConfig.Session` (`AdminConfig`,
  `SessionConfig`) fields in `src/config/config.go` were removed along
  with it, since they existed only to support the deleted login. No
  `server.yml` example docs elsewhere in the repo referenced these fields,
  so no further doc cleanup is needed.

## Makefile Convention Gaps

- `Makefile` `PROJECTNAME`/`PROJECTORG` are hardcoded (`anime`/`apimgr`)
  instead of inferred from `git remote get-url origin` per convention.
- `Makefile` `build` target's `LDFLAGS` is missing `-trimpath`.
- `Makefile` is missing the required `release` and `dev` targets.
  Out of scope for the CI-fix/CommitID-rename pass that found this
  (go-lint flagged it as pre-existing); needs a dedicated Makefile pass.
