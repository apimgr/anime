# Configuration

The server creates `server.yml` on first run under its configuration directory. The default directory is selected by `src/paths`; override it with `--config` or `CONFIG_DIR`. Runtime data and logs can be overridden with `--data`/`DATA_DIR` and `--log`/`LOGS_DIR`.

Server listen settings can be overridden in this order: command-line flags, environment, then `server.yml`. `PORT` defaults to `8080` outside the container. The container sets its listen port to `80`.

The CLI needs a server URL from `--server` or `ANIME_SERVER_PRIMARY`. No official site is configured in this repository. The CLI stores configuration in the invoking user's config directory; do not put credentials or private data in a shared system directory.
