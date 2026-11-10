# Development

The project uses Go under `src/`. Follow the repository's `AI.md` and `IDEA.md` as the implementation and product specifications. Build and validate Go code through Makefile targets using the `casjaysdev/go:latest` container.

```sh
make dev
make test
make local
make build
```

Tests must keep runtime data in an OS temporary directory. Do not use production or development compose files for AI validation; use the dedicated test workflow from the primary session. CI builds use the Go container directly rather than the Makefile.
