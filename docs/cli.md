# CLI reference

The companion `anime-cli` binary reads only from the public API.

```sh
anime-cli --server https://anime.example random
anime-cli --server https://anime.example list
anime-cli --server https://anime.example search "fullmetal alchemist"
```

`--server URL` overrides `ANIME_SERVER_PRIMARY`, which overrides `server.primary` in the user-scoped `cli.yml`. Use `--config PROFILE` to select a named profile (`PROFILE.yml`) or an explicit YAML path. First run creates user-only config/data/cache/log directories and a 0600 config file. With no command on an interactive terminal, the client starts its quote browser TUI when `tui.enabled` is true. Its theme comes from `tui.theme` (`dark`, `light`, or `system`).

`--output`/`--format` accepts `table`, `json`, `yaml`, `plain`, or `csv`; the default is selected by `output.format` in `cli.yml` and can be overridden with `ANIME_OUTPUT_FORMAT`. Server API version, timeout, retry count, and retry delay are configurable under `server`. Non-random responses are cached in the user cache directory according to `cache.enabled`, `cache.ttl`, and `cache.max_size`. The CLI writes a private `cli.log` file in the user log directory; `logging.level`, `logging.file`, `logging.max_size`, and `logging.max_files` configure its verbosity and rotation. `--color` respects `NO_COLOR`; `--lang` falls back to English when the requested language is not available. Use `--shell completions [bash|zsh|fish|sh|dash|ksh|powershell|pwsh]` or `--shell init [SHELL]` for shell integration.
