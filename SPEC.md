# Anime Quotes API Server - Project Specification

**Project**: anime
**Module**: github.com/apimgr/anime
**Language**: Go 1.23+
**Purpose**: REST API serving 2000+ curated anime quotes
**Data**: Embedded JSON (src/data/anime.json)
**Organization**: apimgr
**Registry**: ghcr.io/apimgr/anime

---

## Compliance

This project follows **BASE.md** specification from the apimgr organization.

### Key Compliance Points

| Requirement | Status | Implementation |
|-------------|--------|----------------|
| Go-based single binary | ✅ | CGO_ENABLED=0, embedded assets |
| File-based YAML config | ✅ | /etc/apimgr/anime/server.yaml |
| No authentication | ✅ | All endpoints public |
| No database | ✅ | File-based config only |
| Tini init in Docker | ✅ | /sbin/tini as entrypoint |
| CLI commands | ✅ | --service, --maintenance |
| REST API | ✅ | /api/v1/* endpoints |
| .txt extension support | ✅ | /api/v1/random.txt, etc. |
| PWA support | ✅ | manifest.json, service worker |
| robots.txt/security.txt | ✅ | Config-based generation |
| Multi-platform | ✅ | Linux, macOS, Windows, BSD (amd64/arm64) |

---

## Directory Structure

```
anime/
├── SPEC.md                    # This file
├── README.md                  # User documentation
├── Dockerfile                 # Alpine + tini
├── docker-compose.yml         # Production deployment
├── Makefile                   # Build system
├── go.mod / go.sum            # Go modules
├── release.txt                # Version tracking
├── scripts/                   # Installation scripts
│   ├── install-linux.sh
│   ├── install-macos.sh
│   ├── install-bsd.sh
│   └── install-windows.ps1
├── docs/                      # MkDocs documentation
└── src/                       # Source code
    ├── main.go                # Entry point
    ├── data/
    │   └── anime.json         # Embedded quote data
    ├── anime/                 # Quote service
    │   └── service.go
    ├── config/                # YAML configuration
    │   └── config.go
    ├── paths/                 # OS-specific paths
    │   └── paths.go
    └── server/                # HTTP server
        ├── server.go
        ├── handlers.go
        ├── middleware.go
        ├── templates.go
        ├── templates/         # Embedded HTML
        └── static/            # Embedded CSS/JS
```

---

## Configuration

**Location**: `/etc/apimgr/anime/server.yaml` (root) or `~/.config/apimgr/anime/server.yaml` (user)

```yaml
server:
  port: ""
  fqdn: ""
  address: "0.0.0.0"
  schedule:
    enabled: true
    notifications: "hourly"
  metrics:
    enabled: false
    endpoint: "/metrics"
  logging:
    access_format: "apache"
    level: "info"

web-ui:
  theme: "dark"
  logo: ""
  favicon: ""
  notifications:
    enabled: true
    announcements: []

web-robots:
  allow: ["/", "/api"]
  deny: ["/debug"]

web-security:
  admin: ""
  cors: "*"
```

---

## CLI Commands

```bash
# Basic
anime --help                  # Show help
anime --version               # Show version
anime --status                # Check if running

# Server
anime                         # Start with defaults
anime --port 8080             # Start on specific port
anime --config /path/to/dir   # Custom config directory

# Service management
anime --service start         # Start service
anime --service stop          # Stop service
anime --service restart       # Restart service
anime --service reload        # Reload configuration
anime --service status        # Service status
anime --service --install     # Install as service
anime --service --uninstall   # Remove service

# Maintenance
anime --maintenance backup [file]    # Backup config/data
anime --maintenance restore [file]   # Restore from backup
anime --maintenance update           # Check and install updates
```

---

## API Endpoints

### Public Endpoints (All routes - NO AUTH)

```
# Web Pages
GET  /                           Home page
GET  /healthz                    Health check

# Special Files
GET  /robots.txt                 Robots file (from config)
GET  /security.txt               Security contact
GET  /manifest.json              PWA manifest
GET  /sw.js                      Service worker

# API v1 - JSON responses
GET  /api/v1/random              Get random quote
GET  /api/v1/quotes              Get all quotes
GET  /api/v1/health              Health check
GET  /api/v1/stats               Statistics

# API v1 - Text responses (.txt extension)
GET  /api/v1/random.txt          Random quote as text
GET  /api/v1/quotes.txt          All quotes as text
GET  /api/v1/health.txt          Health as text
GET  /api/v1/stats.txt           Statistics as text
```

---

## Data Source

### anime.json (Embedded)
- **Location**: src/data/anime.json
- **Records**: 2000+ anime quotes
- **Format**: JSON array
- **Fields**: quote, character, anime

```json
[
  {
    "anime": "Fullmetal Alchemist",
    "character": "Edward Elric",
    "quote": "A lesson without pain is meaningless."
  }
]
```

---

## Docker

```bash
# Production
docker run -d \
  --name anime \
  -p 8080:80 \
  -v ./config:/config \
  ghcr.io/apimgr/anime:latest

# Docker Compose
docker-compose up -d
```

**Image Details**:
- Base: Alpine + tini
- Size: ~17MB
- Port: 80 (internal)
- User: nobody (65534)
- Entrypoint: /sbin/tini --
- Healthcheck: anime --status

---

## Build

```bash
# All platforms
make build

# Test
make test

# Docker image
make docker-dev

# Release
make release
```

**Output Binaries**:
- anime-linux-amd64
- anime-linux-arm64
- anime-macos-amd64
- anime-macos-arm64
- anime-windows-amd64.exe
- anime-windows-arm64.exe
- anime-bsd-amd64
- anime-bsd-arm64

---

## Rate Limiting

| Scope | Requests/Second | Burst |
|-------|-----------------|-------|
| Global | 100 | 200 |
| API | 50 | 100 |

---

## Security Headers

```
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'; ...
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

---

## License

MIT License

**Data License**: Various (fair use - quotes)
