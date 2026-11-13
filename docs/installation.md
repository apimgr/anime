# Installation

## Docker Compose

Review `docker/docker-compose.yml`, especially its published port and data paths, then start it with:

```sh
docker compose -f docker/docker-compose.yml up -d
```

The application listens on port 80 in the container.
The production Compose file enables a Tor hidden service and persists its address in the mounted data volume. Once Tor publishes the address, `/server/help` displays it. Set `ENABLE_TOR=false` to disable Tor.

## Binary

Download the matching `anime-{os}-{arch}` release artifact, install it on the target host, and run `anime --help`. Use `anime --version` to inspect build information. By default, the server opens a local SQLite database under its database directory and embeds the quote collection in the binary. Operators can configure a remote libSQL database with `server.database.driver` and `server.database.url`.

The companion `anime-cli-{os}-{arch}` binary calls an existing service; pass `--server` or set `ANIME_SERVER_PRIMARY`.

Prebuilt server and CLI artifacts are published for Linux, macOS, Windows, and FreeBSD on amd64 and arm64. The installer scripts are in `scripts/` for Linux, macOS, BSD, and Windows PowerShell. Review a script before running it, and use `scripts/uninstall.sh` to remove supported installations.

### Updates

Check for a release or install it with checksum verification:

```sh
anime --update check
anime --update yes
anime --update branch stable
```

The update channel is stored in `server.yml`; `beta` includes stable releases, and `devel` also includes beta and rolling releases. `--maintenance update` is an alias for installing the current channel's update.
