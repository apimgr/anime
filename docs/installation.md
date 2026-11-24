# Installation

## Docker Compose

Review `docker/docker-compose.yml`, especially its published port and data paths, then start it with:

```sh
docker compose -f docker/docker-compose.yml up -d
```

The application listens on port 80 in the container.
The application starts Tor automatically when its binary is available and persists its address in the mounted data volume. I2P is enabled by default in container mode. Published overlay addresses and health appear on `/server/help`; configure I2P under `server.i2p`.

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

## System service

Install the binary and run the service installer with administrative privileges:

```sh
sudo anime --service --install
systemctl status anime
```

The installer detects systemd, OpenRC, runit, s6, SysVinit, launchd, Windows Service, or FreeBSD rc.d. Unix system services start with elevated privileges only long enough to create the `anime` system account, prepare owned directories, and bind ports below 1024; the process then drops groups and UID before initializing the application. The Windows service uses the `NT SERVICE\anime` virtual service account. A non-administrator can install a user service on systemd (`systemctl --user`) or launchd (LaunchAgent); user mode cannot bind ports below 1024. Service actions are `--service start|stop|restart|reload`, `--service --disable`, and `--service --uninstall`. Inspect service state through the native service manager; `anime --status` checks application health. Uninstall asks before removing configuration, data, backups, logs, and the system service account; the binary remains installed.

The service starts as root on Unix so the application can apply a staged self-update before dropping privileges. Linux systemd uses `Restart=on-failure`; launchd, runit, and s6 restart after exit 75. OpenRC, SysVinit, and rc.d leave a staged update pending until the next service start. See the [configuration guide](configuration.md) for scheduled updates and the defer window.

### Updates

Check for a release or install it with checksum verification:

```sh
anime --update check
anime --update yes
anime --update branch stable
```

The update channel is stored in `server.yml`; `beta` includes stable releases, and `devel` also includes beta and rolling releases. `--maintenance update` is an alias for installing the current channel's update.

The daily `update_check` scheduler honors `server.update.defer_days`. With
`auto_install: false` it notifies the operator only. With `auto_install: true`,
a root-started Unix service downloads and checksum-verifies an eligible release,
stages it under the data directory, then exits with status 75 for a supervised
restart and privileged replacement. User services and Windows services remain
notify-only and provide a manual update command.
