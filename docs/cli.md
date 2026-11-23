# CLI reference

The companion `anime-cli` binary reads only from the public API.

```sh
anime-cli --server https://anime.example random
anime-cli --server https://anime.example list
anime-cli --server https://anime.example search "fullmetal alchemist"
```

`--server URL` overrides `ANIME_SERVER_PRIMARY`, which overrides `server.primary` in the user-scoped `cli.yml`. Use `--config PROFILE` to select a named profile (`PROFILE.yml`) or an explicit YAML path. First run creates user-only config/data/cache/log directories and a 0600 config file. With no command and no configured server, an interactive terminal prompts for the server URL, tests `/api/v1/server/healthz`, and saves a working URL before opening the quote browser TUI. Enter `q` to cancel. A no-command invocation without a terminal still requires a configured server. The TUI is enabled by default and can be disabled with `tui.enabled: false`; its theme comes from `tui.theme` (`dark`, `light`, or `system`).

`--output`/`--format` accepts `table`, `json`, `yaml`, `plain`, or `csv`; the default is selected by `output.format` in `cli.yml` and can be overridden with `ANIME_OUTPUT_FORMAT`. Server API version, timeout, retry count, and retry delay are configurable under `server`. Non-random responses are cached in the user cache directory according to `cache.enabled`, `cache.ttl`, and `cache.max_size`. The CLI writes a private `cli.log` file in the user log directory; `logging.level`, `logging.file`, `logging.max_size`, and `logging.max_files` configure its verbosity and rotation. `--color` respects `NO_COLOR`. `--lang` accepts any supported locale, sends it in `Accept-Language` on API and TUI requests, and falls back to English for unsupported values. With `--lang auto` (the default), the client uses `lang` in `cli.yml`, then `LC_ALL`, then `LANG`, then English. Use `--shell completions [bash|zsh|fish|sh|dash|ksh|powershell|pwsh]` or `--shell init [SHELL]` for shell integration.

Set the language in the user-scoped `cli.yml` with a top-level value:

```yaml
lang: fr
```

The setup prompts and main TUI labels are translated for English, Spanish, French, German, Italian, and Arabic. Other supported UI locales fall back to English for these client-only strings.

The CLI checks `/api/autodiscover` on each invocation. An available update prompts in an interactive terminal; scripts continue without changing the installed binary. Check or install explicitly with:

```sh
anime-cli --update check
anime-cli --update yes
```

Updates download the current platform artifact over HTTPS, verify its SHA-256 against discovery metadata, replace the running executable atomically, and restart with the original arguments. The serving instance publishes operator-provisioned release assets from `{data_dir}/cli` by default (configurable with `cli.binary_download.dir`): copy the platform binaries plus `version.txt` and `sha256.txt` from a release into that directory. The server does not fetch or build CLI binaries; an empty or invalid asset directory advertises no update. If the server sets `cli_min_version` above the installed version, normal CLI requests are refused and `--update yes` is required.
