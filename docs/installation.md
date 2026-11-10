# Installation

## Docker Compose

Review `docker/docker-compose.yml`, especially its published port and data paths, then start it with:

```sh
docker compose -f docker/docker-compose.yml up -d
```

The application listens on port 80 in the container.

## Binary

Download the matching static `anime-{os}-{arch}` release artifact, install it on the target host, and run `anime --help`. Use `anime --version` to inspect build information. No database or external runtime service is required.

The companion `anime-cli-{os}-{arch}` binary calls an existing service; pass `--server` or set `ANIME_SERVER_PRIMARY`.

### Updates

Check for a release or install it with checksum verification:

```sh
anime --update check
anime --update yes
anime --update branch stable
```

The update channel is stored in `server.yml`; `beta` includes stable releases, and `devel` also includes beta and rolling releases. `--maintenance update` is an alias for installing the current channel's update.
