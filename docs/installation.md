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

Prebuilt server and CLI artifacts are published for Linux, macOS, Windows, and FreeBSD on amd64 and arm64. Installer scripts are available in `scripts/` for Linux, macOS, BSD, and Windows PowerShell. Review a script before running it. The Linux installer configures the detected init system, including systemd; `scripts/uninstall.sh` removes supported installations after confirmation.

| Operating system | Architecture | Server artifact | CLI artifact |
|---|---|---|---|
| Linux | amd64 | `anime-linux-amd64` | `anime-cli-linux-amd64` |
| Linux | arm64 | `anime-linux-arm64` | `anime-cli-linux-arm64` |
| macOS | amd64 | `anime-darwin-amd64` | `anime-cli-darwin-amd64` |
| macOS | arm64 | `anime-darwin-arm64` | `anime-cli-darwin-arm64` |
| Windows | amd64 | `anime-windows-amd64.exe` | `anime-cli-windows-amd64.exe` |
| Windows | arm64 | `anime-windows-arm64.exe` | `anime-cli-windows-arm64.exe` |
| FreeBSD | amd64 | `anime-freebsd-amd64` | `anime-cli-freebsd-amd64` |
| FreeBSD | arm64 | `anime-freebsd-arm64` | `anime-cli-freebsd-arm64` |

Use the release's published SHA-256 checksums to verify a downloaded artifact before installing it. The interactive self-update command performs checksum verification automatically. The Windows installer downloads the matching server binary; the CLI is a separate artifact.

## systemd service

On a Linux host with systemd, install the downloaded binary and run the service installer as root:

```sh
sudo anime --service --install
sudo systemctl status anime
sudo journalctl -u anime -f
```

The installer creates and enables `anime.service`. Use `systemctl restart anime` to restart it and `systemctl stop anime` to stop it. The service runs as the dedicated `anime` account. Its configuration, data, and log locations follow the paths shown by `anime --help` and the [configuration guide](configuration.md). Review the generated service unit before exposing the server publicly.

### Updates

Check for a release or install it with checksum verification:

```sh
anime --update check
anime --update yes
anime --update branch stable
```

The update channel is stored in `server.yml`; `beta` includes stable releases, and `devel` also includes beta and rolling releases. `--maintenance update` is an alias for installing the current channel's update.
