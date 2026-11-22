# Contributing

## Local setup

This project uses Go and builds through the `casjaysdev/go:latest` container. The Makefile provides the supported development and verification commands:

```sh
make dev
make test
make local
make build
```

`make test` runs `go vet`, `govulncheck`, the Go test suite, and the 60% coverage gate inside the Go container. See [development documentation](../docs/development.md) for details.

## Changes

1. Open an issue or discussion describing the problem and intended change.
2. Create a topic branch and submit a pull request to `main`.
3. Include tests for behavior changes and update the relevant user or operator documentation.
4. Do not include credentials, private data, or generated build artifacts.
5. Report security vulnerabilities through [GitHub private vulnerability reporting](https://github.com/apimgr/anime/security/advisories/new), not a public issue.

Pull requests must pass the repository's CI checks before merging.
