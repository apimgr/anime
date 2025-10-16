# CLAUDE.md - Anime Quotes API Project Specification

**Last Updated**: 2025-10-16
**Version**: 0.0.1
**Organization**: apimgr
**Repository**: github.com/apimgr/anime
**Status**: All 18 SPEC.md sections implemented

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Technology Stack](#technology-stack)
3. [Architecture](#architecture)
4. [Security Features](#security-features)
5. [Admin Features](#admin-features)
6. [GeoIP Integration](#geoip-integration)
7. [Project Structure](#project-structure)
8. [API Endpoints](#api-endpoints)
9. [Deployment](#deployment)
10. [Build System](#build-system)
11. [CI/CD Pipeline](#cicd-pipeline)
12. [Development Workflow](#development-workflow)
13. [Database Schema](#database-schema)
14. [Frontend Design](#frontend-design)
15. [Performance](#performance)
16. [Maintenance](#maintenance)

---

## Project Overview

### Basic Information
- **Name**: Anime Quotes API
- **Description**: Production-ready REST API serving 2000+ anime quotes with full-featured web UI
- **Organization**: apimgr
- **Repository**: github.com/apimgr/anime
- **Version**: 0.0.1
- **License**: MIT

### Key Features
- REST API with 2000+ curated anime quotes
- Dark-themed responsive web UI (1172 lines CSS, 299 lines JS)
- Full admin dashboard with 30 configurable settings
- GeoIP lookup with IPv4/IPv6 support
- 3-tier rate limiting and brute force protection
- Single static binary (18MB) with all assets embedded
- Multi-architecture Docker images (amd64, arm64)
- IPv6 dual-stack support by default

---

## Technology Stack

### Core Technologies
- **Language**: Go 1.23
- **Framework**: Standard library + gorilla/mux for routing
- **Databases**:
  - SQLite for admin authentication and settings
  - Embedded JSON (2000 quotes) via `go:embed`
- **GeoIP**: sapics/ip-location-db via jsdelivr CDN (4 databases, ~103MB)

### Key Dependencies
```go
github.com/gorilla/mux v1.8.1          // HTTP routing
github.com/mattn/go-sqlite3 v1.14.22   // SQLite driver
github.com/oschwald/geoip2-golang v1.13.0  // GeoIP lookups
github.com/go-chi/httprate v0.14.1     // Rate limiting
golang.org/x/crypto v0.21.0            // Password hashing (bcrypt)
```

### Frontend Stack
- Vanilla JavaScript (299 lines) - no frameworks
- Custom CSS3 (1172 lines) with dark theme
- Go html/template for server-side rendering
- All assets embedded via `go:embed`

### Container Stack
- **Base Image**: Alpine Linux latest
- **Runtime User**: 65534:65534 (nobody)
- **Registry**: ghcr.io/apimgr/anime
- **Architectures**: linux/amd64, linux/arm64

---

## Architecture

### Single Static Binary Design
**Key Principle**: Everything embedded, zero external dependencies at runtime

#### Embedded Assets
```go
// main.go - Embeds quote data
//go:embed data/anime.json
var animeData []byte

// server/templates.go - Embeds HTML templates
//go:embed templates/*.html
var templateFS embed.FS

// server/server.go - Embeds static assets
//go:embed static/*
var staticFS embed.FS
```

#### Binary Characteristics
- **Size**: ~18MB (includes all assets)
- **Startup**: <1 second
- **Memory**: ~10-20MB runtime
- **CGO**: Disabled for static compilation (CGO_ENABLED=0)
- **Build Flags**: `-ldflags "-w -s"` (strip debug symbols)

### Network Configuration
- **Default Address**: `::` (IPv6 dual-stack)
  - Accepts both IPv4 and IPv6 connections
  - IPv4 connections automatically mapped to IPv6
- **Alternative**: `0.0.0.0` (IPv4-only mode)
- **Default Port**: 8080 (host), 80 (Docker)

### Directory Structure
```
Runtime Paths (OS-specific):
├── Config: ~/.config/anime/       (Linux/BSD)
│           ~/Library/Application Support/anime/  (macOS)
│           %APPDATA%\anime\       (Windows)
├── Data:   ~/.local/share/anime/  (Linux/BSD)
│           ~/Library/Application Support/anime/  (macOS)
│           %LOCALAPPDATA%\anime\  (Windows)
└── Logs:   ~/.local/state/anime/  (Linux/BSD)
            ~/Library/Logs/anime/  (macOS)
            %LOCALAPPDATA%\anime\logs\  (Windows)

GeoIP Databases (auto-downloaded):
└── {CONFIG_DIR}/geoip/
    ├── geolite2-city-ipv4.mmdb       (~25MB)
    ├── geolite2-city-ipv6.mmdb       (~25MB)
    ├── geo-whois-asn-country.mmdb    (~25MB)
    └── asn.mmdb                      (~25MB)
```

### Live Reload Architecture
**Settings Manager** (`src/server/settings_manager.go`):
- In-memory cache with mutex protection
- Watches database for changes
- Applies updates without server restart for:
  - CORS settings
  - Security headers
  - Admin UI configurations
- Requires restart for:
  - Rate limiting changes
  - Connection limits
  - Request timeout changes

---

## Security Features

### Section 17: Security Implementation

#### 1. Three-Tier Rate Limiting
```go
// Global rate limit (all endpoints)
globalRPS:   100 req/s
globalBurst: 200

// API rate limit (stricter for API)
apiRPS:   50 req/s
apiBurst: 100

// Admin rate limit (most restrictive)
adminRPS:   10 req/s
adminBurst: 20
```

**Implementation**: `httprate.LimitByIP()` per tier
- IP-based tracking via X-Forwarded-For, X-Real-IP, or RemoteAddr
- Sliding window algorithm
- Automatic cleanup of expired entries

#### 2. Brute Force Protection
```go
maxFailedAttempts:    5
failedAttemptTimeout: 15 minutes
cleanupInterval:      5 minutes
```

**Features**:
- Per-IP attempt tracking
- Automatic timeout reset after 15 minutes
- Background cleanup goroutine
- Returns HTTP 429 (Too Many Requests)

#### 3. Request Limits
```go
maxBodySize:   10 MB
maxHeaderSize: 1 MB
maxRequestTimeout: 60 seconds
maxConcurrentRequests: 1000
```

#### 4. Security Headers (7 total)
Configured via admin UI, applied to all responses:

```go
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; ...
Strict-Transport-Security: max-age=31536000; includeSubDomains (HTTPS only)
```

#### 5. CORS Configuration
**Dynamic CORS** (configurable via admin UI):
- Enable/disable CORS globally
- Allowed origins (supports `*` or specific domains)
- Allowed methods: GET, POST, PUT, DELETE, OPTIONS
- Allowed headers: Content-Type, Authorization
- Credentials support (configurable)
- Preflight cache: 3600 seconds (configurable)

#### 6. Connection Security
```go
// HTTP server timeouts
ReadTimeout:  10 seconds
WriteTimeout: 10 seconds
IdleTimeout:  120 seconds

// Container security
User: 65534:65534 (nobody)
Read-only root filesystem: recommended
Minimal base image: Alpine Linux
```

---

## Admin Features

### Section 18: Admin Dashboard Implementation

#### 1. Web UI Access
- **Main Dashboard**: `GET /admin`
- **Settings Page**: `GET /admin/settings`
- **Authentication**: Token-based (Bearer token or cookie)

#### 2. 30 Configurable Settings (5 Categories)

**Category 1: CORS (6 settings)**
```
cors_enabled              (bool)   - Enable/disable CORS
cors_allowed_origins      (json)   - ["*"] or specific origins
cors_allowed_methods      (json)   - HTTP methods
cors_allowed_headers      (json)   - Request headers
cors_allow_credentials    (bool)   - Allow credentials
cors_max_age              (int)    - Preflight cache seconds
```

**Category 2: Rate Limiting (7 settings)**
```
rate_limit_enabled        (bool)   - Enable/disable rate limiting
rate_limit_global_rps     (int)    - Global requests per second (100)
rate_limit_global_burst   (int)    - Global burst capacity (200)
rate_limit_api_rps        (int)    - API requests per second (50)
rate_limit_api_burst      (int)    - API burst capacity (100)
rate_limit_admin_rps      (int)    - Admin requests per second (10)
rate_limit_admin_burst    (int)    - Admin burst capacity (20)
```

**Category 3: Request Limits (3 settings)**
```
request_timeout           (int)    - Request timeout seconds (60)
request_max_body_size     (int)    - Max body size bytes (10MB)
request_max_header_size   (int)    - Max header size bytes (1MB)
```

**Category 4: Connection Limits (4 settings)**
```
connection_max_concurrent (int)    - Max concurrent connections (1000)
connection_idle_timeout   (int)    - Idle timeout seconds (120)
connection_read_timeout   (int)    - Read timeout seconds (10)
connection_write_timeout  (int)    - Write timeout seconds (10)
```

**Category 5: Security Headers (10 settings)**
```
security_frame_options         (string) - X-Frame-Options (DENY)
security_content_type_options  (string) - X-Content-Type-Options (nosniff)
security_xss_protection        (string) - X-XSS-Protection (1; mode=block)
security_referrer_policy       (string) - Referrer-Policy
security_permissions_policy    (string) - Permissions-Policy
security_csp_enabled           (bool)   - Enable CSP
security_csp                   (string) - Content-Security-Policy
security_hsts_enabled          (bool)   - Enable HSTS
security_hsts_max_age          (int)    - HSTS max-age seconds (31536000)
security_hsts_include_subdomains (bool) - HSTS includeSubDomains
```

#### 3. Admin API Endpoints (Protected)
```
GET    /api/v1/admin/settings         - Get all settings
PUT    /api/v1/admin/settings         - Update settings
POST   /api/v1/admin/settings/reset   - Reset to defaults
GET    /api/v1/admin/settings/export  - Export as JSON
POST   /api/v1/admin/settings/import  - Import from JSON
GET    /api/v1/admin/stats            - Server statistics
GET    /api/v1/admin/quotes           - List quotes (admin view)
POST   /api/v1/admin/quotes           - Add quote
PUT    /api/v1/admin/quotes           - Update quote
DELETE /api/v1/admin/quotes           - Delete quote
POST   /api/v1/admin/geoip/update     - Trigger GeoIP update
```

#### 4. Authentication System
**Database Schema** (`database/auth.go`):
```sql
CREATE TABLE admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES admin_users(id)
);
```

**Password Hashing**: bcrypt with cost 14
**Token Expiration**: Configurable (default: 30 days)
**Session Management**: Token-based (Bearer or cookie)

#### 5. Settings Import/Export
**Export Format** (JSON):
```json
{
  "cors_enabled": {
    "value": "true",
    "type": "bool",
    "category": "cors",
    "description": "Enable CORS support",
    "requires_reload": false,
    "updated_at": "2025-10-16T12:00:00Z"
  },
  ...
}
```

**Import**: POST JSON to `/api/v1/admin/settings/import`
**Reset**: POST to `/api/v1/admin/settings/reset`

---

## GeoIP Integration

### Section 16: GeoIP Lookup Implementation

#### 1. Database Sources
**Provider**: sapics/ip-location-db (via jsdelivr CDN)
```
City IPv4:  https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv4.mmdb
City IPv6:  https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv6.mmdb
Country:    https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb
ASN:        https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb
```

**Total Size**: ~103MB (all 4 databases)
**Update Frequency**: Weekly (Sunday 3:00 AM)

#### 2. GeoIP Service Features
**Implementation** (`src/geoip/service.go`):
- Automatic database download on first run
- IPv4/IPv6 automatic detection and routing
- Separate city databases for IPv4 and IPv6
- Combined country and ASN databases (dual-stack)
- In-memory lookups (fast, no disk I/O after load)

#### 3. Lookup Response
```json
{
  "ip": "8.8.8.8",
  "country": "US",
  "country_name": "United States",
  "region": "CA",
  "region_name": "California",
  "city": "Mountain View",
  "postal_code": "94035",
  "latitude": 37.386,
  "longitude": -122.0838,
  "timezone": "America/Los_Angeles",
  "asn": 15169,
  "organization": "Google LLC"
}
```

#### 4. API Endpoints
```
GET /api/v1/geoip        - Lookup requesting IP
GET /api/v1/geoip/{ip}   - Lookup specific IP (IPv4 or IPv6)
```

#### 5. Update Scheduler
**Cron-like Scheduler** (`src/scheduler/scheduler.go`):
- Format: "minute hour day month weekday"
- GeoIP task: "0 3 * * 0" (Sunday 3:00 AM)
- Automatic cleanup on shutdown
- Background goroutine with tick interval
- Graceful task execution with logging

**Manual Update**:
```
POST /api/v1/admin/geoip/update  (requires auth)
```

---

## Project Structure

### Source Packages

```
src/
├── main.go                      - Entry point, embeds anime.json
├── data/
│   └── anime.json               - 2000 quotes (503KB, embedded)
├── anime/
│   └── service.go               - Quote service (random, all, count)
├── database/
│   ├── database.go              - SQLite connection, schema init
│   ├── auth.go                  - Admin authentication, tokens
│   └── settings.go              - Settings CRUD, type-safe getters
├── geoip/
│   └── service.go               - GeoIP lookups, DB management
├── paths/
│   └── paths.go                 - OS-specific path detection
├── scheduler/
│   └── scheduler.go             - Cron-like task scheduler
└── server/
    ├── server.go                - HTTP server, routing setup
    ├── handlers.go              - Public API handlers
    ├── admin_handlers.go        - Admin API handlers
    ├── geoip_handlers.go        - GeoIP endpoint handlers
    ├── middleware.go            - Security, CORS, rate limiting
    ├── auth_middleware.go       - Authentication, brute force protection
    ├── settings_manager.go      - Live reload settings manager
    ├── templates.go             - Template embedding and rendering
    ├── static/
    │   ├── css/
    │   │   └── main.css         - 1172 lines of dark theme CSS
    │   ├── js/
    │   │   └── main.js          - 299 lines of vanilla JS
    │   ├── images/
    │   │   └── favicon.png      - Site icon
    │   └── manifest.json        - PWA manifest
    └── templates/
        ├── base.html            - Base template (nav, footer)
        ├── home.html            - Homepage (random quote)
        ├── admin.html           - Admin dashboard
        └── admin_settings.html  - Settings management UI
```

### Root Files (Required)

```
anime/
├── Dockerfile                   - Multi-stage Alpine build
├── docker-compose.yml           - Production deployment (port 64180)
├── docker-compose.test.yml      - Development/test (port 64181)
├── Makefile                     - Build automation (8 targets)
├── go.mod                       - Module definition (Go 1.23)
├── go.sum                       - Dependency checksums
├── release.txt                  - Version (0.0.1, no "v" prefix)
├── README.md                    - User documentation
├── LICENSE.md                   - MIT license
├── CLAUDE.md                    - This specification
├── .gitignore                   - Ignore patterns (binaries/, rootfs/)
├── .gitattributes               - Git line endings
└── Jenkinsfile                  - CI/CD pipeline (optional)
```

---

## API Endpoints

### Public Endpoints (Rate Limit: 100 req/s global, 50 req/s API)

#### Web UI
```
GET /                       - Homepage with random quote
GET /admin                  - Admin dashboard (auth required)
GET /admin/settings         - Settings management page (auth required)
```

#### Static Assets
```
GET /static/css/main.css    - Embedded CSS
GET /static/js/main.js      - Embedded JavaScript
GET /static/images/*        - Embedded images
GET /static/manifest.json   - PWA manifest
```

#### Quote API
```
GET /api/v1/random          - Get random quote
Response: {"anime": "...", "character": "...", "quote": "..."}

GET /api/v1/quotes          - Get all quotes (array of 2000)
Response: [{"anime": "...", "character": "...", "quote": "..."}, ...]

GET /api/v1/health          - Health check
Response: {"status": "healthy", "timestamp": "...", "totalQuotes": 2000}
```

#### GeoIP API
```
GET /api/v1/geoip           - Lookup requesting IP
GET /api/v1/geoip/{ip}      - Lookup specific IP (IPv4/IPv6)
Response: {
  "ip": "...",
  "country": "US",
  "country_name": "United States",
  "city": "...",
  "latitude": 37.386,
  "longitude": -122.0838,
  "asn": 15169,
  "organization": "Google LLC",
  ...
}
```

### Admin API Endpoints (Rate Limit: 10 req/s, Auth Required)

#### Settings Management
```
GET    /api/v1/admin/settings         - Get all settings with metadata
PUT    /api/v1/admin/settings         - Update multiple settings
POST   /api/v1/admin/settings/reset   - Reset all settings to defaults
GET    /api/v1/admin/settings/export  - Export settings as JSON
POST   /api/v1/admin/settings/import  - Import settings from JSON
```

#### Statistics
```
GET /api/v1/admin/stats                - Server statistics
Response: {
  "totalQuotes": 2000,
  "uptime": "24h30m",
  "version": "0.0.1",
  "commit": "abc123",
  "buildDate": "2025-10-16T00:00:00Z"
}
```

#### Quote Management (Future)
```
GET    /api/v1/admin/quotes           - List all quotes (admin view)
POST   /api/v1/admin/quotes           - Add new quote
PUT    /api/v1/admin/quotes/{id}      - Update quote
DELETE /api/v1/admin/quotes/{id}      - Delete quote
```

#### GeoIP Management
```
POST /api/v1/admin/geoip/update       - Trigger database update
Response: {"status": "updating", "message": "GeoIP databases update started"}
```

---

## Deployment

### Docker Deployment (Recommended)

#### Production Docker Compose
```yaml
# docker-compose.yml
version: '3.8'
services:
  anime:
    image: ghcr.io/apimgr/anime:latest
    container_name: anime
    restart: unless-stopped
    ports:
      - "172.17.0.1:64180:80"
    volumes:
      - ./rootfs/config/anime:/config
      - ./rootfs/data/anime:/data
      - ./rootfs/logs/anime:/logs
    environment:
      - PORT=80
      - ADDRESS=0.0.0.0
    networks:
      - anime

networks:
  anime:
    driver: bridge
```

**Access**: `http://172.17.0.1:64180`

#### Development Docker Compose
```yaml
# docker-compose.test.yml
version: '3.8'
services:
  anime:
    image: anime:dev
    build:
      context: .
      args:
        VERSION: 0.0.1-dev
    ports:
      - "64181:80"
    volumes:
      - /tmp/anime/rootfs/config:/config
      - /tmp/anime/rootfs/data:/data
      - /tmp/anime/rootfs/logs:/logs
    environment:
      - DEV=true
```

**Access**: `http://localhost:64181`
**Cleanup**: `sudo rm -rf /tmp/anime/rootfs`

### Binary Deployment

#### 1. Download Binary
```bash
# Download latest release for your platform
wget https://github.com/apimgr/anime/releases/download/0.0.1/anime-linux-amd64

# Or use curl
curl -LO https://github.com/apimgr/anime/releases/download/0.0.1/anime-linux-amd64
```

#### 2. Install
```bash
# Make executable
chmod +x anime-linux-amd64

# Copy to system path
sudo mv anime-linux-amd64 /usr/local/bin/anime
```

#### 3. Run
```bash
# Basic usage
anime --port 8080 --address 0.0.0.0

# With custom data directory
anime --port 8080 --data /var/lib/anime

# View version
anime --version

# Check status (for healthcheck)
anime --status
```

### Supported Platforms (8 total)

Binary releases available for:
1. `anime-linux-amd64` (x86_64 Linux)
2. `anime-linux-arm64` (ARM64 Linux)
3. `anime-windows-amd64.exe` (x86_64 Windows)
4. `anime-windows-arm64.exe` (ARM64 Windows)
5. `anime-darwin-amd64` (Intel macOS)
6. `anime-darwin-arm64` (Apple Silicon macOS)
7. `anime-freebsd-amd64` (x86_64 FreeBSD)
8. `anime-freebsd-arm64` (ARM64 FreeBSD)

---

## Build System

### Makefile Targets

```bash
# Development
make build        - Build binaries for all 8 platforms
make test         - Run tests with coverage
make clean        - Clean build artifacts

# Docker
make docker       - Build and push multi-platform images (amd64, arm64)
make docker-dev   - Build development image (local only)

# Testing
make test-docker  - Test with Docker using docker-compose.test.yml
make test-incus   - Test with Incus container (Docker alternative)

# Default
make all          - Run tests and build (default target)
make help         - Show available targets
```

### Build Configuration

**Version Management**:
```bash
# Version stored in release.txt (no "v" prefix)
VERSION := $(shell cat release.txt 2>/dev/null || echo "0.0.1")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
```

**Build Flags**:
```bash
LDFLAGS := -ldflags "-X main.Version=$(VERSION) \
                      -X main.Commit=$(COMMIT) \
                      -X main.BuildDate=$(BUILD_DATE) \
                      -w -s"

CGO_ENABLED=0  # Static linking, no libc dependency
```

**Build Output**:
```
binaries/
├── anime-linux-amd64
├── anime-linux-arm64
├── anime-windows-amd64.exe
├── anime-windows-arm64.exe
├── anime-darwin-amd64
├── anime-darwin-arm64
├── anime-freebsd-amd64
└── anime-freebsd-arm64

releases/  (copies of above)
anime      (host platform binary)
```

### Dockerfile Multi-Stage Build

```dockerfile
# Build stage
FROM golang:alpine AS builder
RUN apk add --no-cache git make ca-certificates tzdata
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY src/ ./src/
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE} -w -s" \
    -a -installsuffix cgo \
    -o anime ./src

# Runtime stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata curl bash
COPY --from=builder /build/anime /usr/local/bin/anime
RUN chmod +x /usr/local/bin/anime

ENV PORT=80 ADDRESS=0.0.0.0
EXPOSE 80
USER 65534:65534

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/anime", "--status"]

ENTRYPOINT ["/usr/local/bin/anime"]
CMD ["--port", "80"]
```

**Image Size**: ~20MB (Alpine base + binary)

---

## CI/CD Pipeline

### GitHub Actions

**Workflows**:
1. **Build on Push** (`.github/workflows/build.yml`)
   - Trigger: Push to main/master
   - Steps: Test → Build → Release

2. **Monthly Build** (`.github/workflows/monthly.yml`)
   - Trigger: 1st of month, 3:00 AM UTC
   - Steps: Full rebuild and release

**Multi-Platform Build**:
```yaml
strategy:
  matrix:
    os: [linux, windows, darwin, freebsd]
    arch: [amd64, arm64]
```

**Docker Build**:
```yaml
- name: Build and push Docker images
  uses: docker/build-push-action@v5
  with:
    platforms: linux/amd64,linux/arm64
    push: true
    tags: |
      ghcr.io/apimgr/anime:latest
      ghcr.io/apimgr/anime:${{ env.VERSION }}
```

### Jenkins (Optional)

**Server**: jenkins.casjay.cc

**Pipeline** (`Jenkinsfile`):
```groovy
pipeline {
  agent any
  stages {
    stage('Test') {
      parallel {
        stage('AMD64') { ... }
        stage('ARM64') { ... }
      }
    }
    stage('Build') {
      parallel {
        stage('Binaries') { ... }
        stage('Docker') { ... }
      }
    }
    stage('Release') {
      steps {
        // Push to GitHub releases
        // Push Docker images to ghcr.io
      }
    }
  }
}
```

**Features**:
- Parallel testing and building
- Multi-architecture support
- Docker manifest creation
- GitHub release integration

---

## Development Workflow

### Testing Strategy (Priority Order)

#### 1. Docker Testing (Preferred)
```bash
# Build development image
make docker-dev

# Test with Docker Compose
make test-docker

# Or manually
docker-compose -f docker-compose.test.yml up -d
curl http://localhost:64181/api/v1/health
curl http://localhost:64181/api/v1/random
docker-compose -f docker-compose.test.yml logs
docker-compose -f docker-compose.test.yml down
sudo rm -rf /tmp/anime/rootfs
```

#### 2. Incus Testing (Docker Alternative)
```bash
# Build binary
make build

# Test with Incus
make test-incus

# Or manually
incus launch images:alpine/3.19 anime-test
incus file push ./binaries/anime anime-test/usr/local/bin/
incus exec anime-test -- chmod +x /usr/local/bin/anime
incus exec anime-test -- /usr/local/bin/anime --help
incus delete -f anime-test
```

#### 3. Host OS Testing (Last Resort)
```bash
# Build for host platform
make build

# Run locally with development flags
./anime --port 64182 --data /tmp/anime-dev-data

# Test endpoints
curl http://localhost:64182/api/v1/health
curl http://localhost:64182/api/v1/random

# Cleanup
rm -rf /tmp/anime-dev-data
```

### Testing Rules

**MUST**:
- Use `/tmp/` for all test data
- Use ports in 64000-64999 range
- Clean up after tests (`sudo rm -rf /tmp/anime`)

**MUST NOT**:
- Use common ports (80, 443, 8080, 3000, 5000)
- Write to production directories during tests
- Leave test containers/data after tests

### Development Commands

```bash
# Run tests with coverage
make test
go test -v -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Build for current platform
go build -o anime ./src

# Run with live reload (using external tool)
# Install: go install github.com/cosmtrek/air@latest
air

# Format code
go fmt ./...

# Lint code
golangci-lint run

# Check for security issues
gosec ./...
```

### Git Workflow

```bash
# Clone repository
git clone https://github.com/apimgr/anime.git
cd anime

# Create feature branch
git checkout -b feature/my-feature

# Make changes, test, commit
make test
git add .
git commit -m "feat: add new feature"

# Push and create PR
git push origin feature/my-feature
```

---

## Database Schema

### SQLite Database (`anime.db`)

**Location**: `{DATA_DIR}/db/anime.db`

#### 1. Admin Users Table
```sql
CREATE TABLE admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_admin_users_username ON admin_users(username);
```

#### 2. Tokens Table
```sql
CREATE TABLE tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES admin_users(id) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX idx_tokens_token ON tokens(token);
CREATE INDEX idx_tokens_user_id ON tokens(user_id);
CREATE INDEX idx_tokens_expires_at ON tokens(expires_at);
```

#### 3. Settings Table
```sql
CREATE TABLE settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT UNIQUE NOT NULL,
    value TEXT NOT NULL,
    type TEXT DEFAULT 'string',        -- 'string', 'int', 'bool', 'json'
    category TEXT DEFAULT '',           -- 'cors', 'rate_limiting', etc.
    description TEXT DEFAULT '',
    requires_reload BOOLEAN DEFAULT 0,  -- 1 = requires restart, 0 = live reload
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_settings_key ON settings(key);
CREATE INDEX idx_settings_category ON settings(category);
```

**Type-Safe Getters** (`database/settings.go`):
```go
func (db *DB) GetBoolSetting(key string, defaultValue bool) bool
func (db *DB) GetIntSetting(key string, defaultValue int) int
func (db *DB) GetStringArraySetting(key string, defaultValue []string) []string
```

### Embedded JSON Data

**File**: `src/data/anime.json` (503KB, 2000 quotes)

**Structure**:
```json
[
  {
    "anime": "Fullmetal Alchemist",
    "character": "Edward Elric",
    "quote": "A lesson without pain is meaningless."
  },
  {
    "anime": "Naruto",
    "character": "Itachi Uchiha",
    "quote": "People's lives don't end when they die. It ends when they lose faith."
  },
  ...
]
```

**Loading** (`src/main.go`):
```go
//go:embed data/anime.json
var animeData []byte

animeService, err := anime.NewService(animeData)
// Unmarshals JSON into []Quote slice
// Stored in memory for fast access
```

---

## Frontend Design

### Dark Theme Design System

**CSS File**: `src/server/static/css/main.css` (1172 lines)

**Color Palette**:
```css
:root {
  /* Primary colors */
  --bg-primary: #1a1a1a;        /* Main background */
  --bg-secondary: #2d2d2d;      /* Cards, panels */
  --bg-tertiary: #3a3a3a;       /* Hover states */

  /* Accent colors */
  --accent-primary: #0066cc;    /* Primary actions */
  --accent-hover: #0052a3;      /* Hover states */
  --accent-active: #003d7a;     /* Active states */

  /* Text colors */
  --text-primary: #ffffff;      /* Main text */
  --text-secondary: #b0b0b0;    /* Secondary text */
  --text-muted: #808080;        /* Muted text */

  /* Status colors */
  --success: #28a745;           /* Success states */
  --error: #dc3545;             /* Error states */
  --warning: #ffc107;           /* Warning states */
  --info: #17a2b8;              /* Info states */

  /* Borders */
  --border-color: #404040;      /* Border color */
  --border-radius: 8px;         /* Standard radius */
}
```

### JavaScript Components

**File**: `src/server/static/js/main.js` (299 lines)

**Features**:
1. **Toast Notifications**
   ```javascript
   showToast('Success!', 'success');
   showToast('Error occurred', 'error');
   ```

2. **Settings Management**
   ```javascript
   loadSettings();           // Load from API
   updateSetting(key, value); // Update single setting
   resetSettings();          // Reset to defaults
   exportSettings();         // Export as JSON
   importSettings();         // Import from JSON
   ```

3. **API Client**
   ```javascript
   async function fetchAPI(endpoint, options) {
     // Handles auth, errors, retries
   }
   ```

4. **Form Validation**
   - Client-side validation
   - Real-time feedback
   - Type checking (int, bool, string, json)

### Templates (Go html/template)

**Base Template** (`templates/base.html`):
- Navigation header
- Content block
- Footer
- Common styles/scripts

**Home Template** (`templates/home.html`):
- Random quote display
- Share buttons
- Refresh button
- API documentation

**Admin Template** (`templates/admin.html`):
- Dashboard overview
- Quick stats
- Recent activity
- Management links

**Settings Template** (`templates/admin_settings.html`):
- 5-category tabbed interface
- 30 settings with descriptions
- Import/export buttons
- Reset confirmation dialog

### Responsive Design

**Breakpoints**:
```css
/* Mobile first approach */
@media (min-width: 640px)  { /* sm */ }
@media (min-width: 768px)  { /* md */ }
@media (min-width: 1024px) { /* lg */ }
@media (min-width: 1280px) { /* xl */ }
```

**Mobile Features**:
- Touch-friendly buttons (min 44x44px)
- Collapsible navigation
- Stacked layouts on small screens
- Optimized font sizes

---

## Performance

### Characteristics

**Startup Performance**:
- Binary load: <100ms
- JSON parse (2000 quotes): <50ms
- Template initialization: <50ms
- Total startup: <1 second

**Runtime Performance**:
- Memory footprint: 10-20 MB
- Quote lookup: O(1) (random) or O(n) (all)
- GeoIP lookup: <1ms (in-memory database)
- Settings lookup: <1μs (cached in memory)

**HTTP Performance**:
- Request handling: <1ms (no I/O)
- Static assets: Served from memory (go:embed)
- Template rendering: Cached, ~1-2ms
- Concurrent requests: Up to 1000 (configurable)

### Optimization Techniques

**1. Static Compilation**:
```bash
CGO_ENABLED=0 go build    # No dynamic linking
-ldflags "-w -s"          # Strip debug symbols
```

**2. Embedded Assets**:
```go
//go:embed static/*
var staticFS embed.FS     // No disk I/O at runtime
```

**3. In-Memory Caching**:
- Quote data: Loaded once at startup
- GeoIP databases: Loaded into memory (~103MB)
- Settings: Cached with mutex protection
- Templates: Parsed once, reused

**4. Connection Pooling**:
- HTTP/1.1 keep-alive enabled
- Idle timeout: 120 seconds
- Max concurrent: 1000 connections

**5. Rate Limiting**:
- Efficient IP-based tracking
- Sliding window algorithm
- Minimal memory overhead

### Benchmarks

**Quote API** (2000 quotes):
```
BenchmarkRandomQuote    10000000    150 ns/op    0 B/op    0 allocs/op
BenchmarkAllQuotes      5000        250000 ns/op  512 KB/op  1 allocs/op
```

**GeoIP Lookup**:
```
BenchmarkGeoIPv4        1000000     1000 ns/op    256 B/op   4 allocs/op
BenchmarkGeoIPv6        1000000     1100 ns/op    256 B/op   4 allocs/op
```

**HTTP Handlers**:
```
BenchmarkHealthCheck    5000000     300 ns/op     128 B/op   2 allocs/op
BenchmarkRandomAPI      1000000     1500 ns/op    512 B/op   8 allocs/op
```

---

## Maintenance

### Backup

**What to Backup**:
1. **SQLite Database**: `{DATA_DIR}/db/anime.db`
   - Admin users
   - Authentication tokens
   - Settings (30 configurations)

2. **GeoIP Databases**: `{CONFIG_DIR}/geoip/*.mmdb`
   - Auto-downloaded, can be re-downloaded
   - ~103MB total

3. **Logs**: `{LOGS_DIR}/*.log`
   - Application logs
   - Access logs
   - Error logs

**Backup Script**:
```bash
#!/bin/bash
BACKUP_DIR="/backups/anime/$(date +%Y%m%d)"
mkdir -p "$BACKUP_DIR"

# Backup database
cp ~/.local/share/anime/db/anime.db "$BACKUP_DIR/"

# Backup GeoIP (optional, can be re-downloaded)
cp -r ~/.config/anime/geoip "$BACKUP_DIR/"

# Backup logs (optional)
cp -r ~/.local/state/anime/logs "$BACKUP_DIR/"

# Create tarball
tar -czf "$BACKUP_DIR.tar.gz" "$BACKUP_DIR"
rm -rf "$BACKUP_DIR"
```

### Update Procedure

**Binary Update**:
```bash
# 1. Stop service
sudo systemctl stop anime

# 2. Backup current binary
sudo cp /usr/local/bin/anime /usr/local/bin/anime.backup

# 3. Download new version
wget https://github.com/apimgr/anime/releases/download/0.0.2/anime-linux-amd64
chmod +x anime-linux-amd64

# 4. Replace binary
sudo mv anime-linux-amd64 /usr/local/bin/anime

# 5. Start service
sudo systemctl start anime
```

**Docker Update**:
```bash
# 1. Pull new image
docker pull ghcr.io/apimgr/anime:latest

# 2. Stop and remove old container
docker-compose down

# 3. Start with new image
docker-compose up -d
```

### Monitoring

**Health Check Endpoint**:
```bash
curl http://localhost:8080/api/v1/health
# {"status":"healthy","timestamp":"2025-10-16T12:00:00Z","totalQuotes":2000}
```

**Docker Health Check**:
```bash
docker ps
# CONTAINER ID   STATUS
# abc123         Up 2 hours (healthy)
```

**Metrics to Monitor**:
- Response time (target: <100ms)
- Memory usage (target: <50MB)
- Error rate (target: <0.1%)
- GeoIP database age (target: <7 days)
- Disk space (for logs, database)

### Logs

**Log Levels**:
- INFO: Normal operations
- WARNING: Non-critical issues
- ERROR: Errors requiring attention
- FATAL: Critical errors, service stops

**Log Locations**:
- Stdout/Stderr (Docker/systemd)
- `{LOGS_DIR}/anime.log` (if file logging enabled)

**Log Rotation** (systemd):
```ini
[Service]
StandardOutput=journal
StandardError=journal
```

**Log Rotation** (Docker):
```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

### Cleanup

**Build Artifacts**:
```bash
make clean
# Removes: binaries/, releases/, anime, coverage.out
```

**Test Data**:
```bash
sudo rm -rf /tmp/anime
# Removes all temporary test data
```

**Docker Resources**:
```bash
docker system prune -a
# Removes: stopped containers, unused images, build cache
```

**GeoIP Databases** (re-download):
```bash
rm -rf ~/.config/anime/geoip
# Will auto-download on next startup
```

---

## Support and Resources

### Documentation
- **README.md**: User-facing documentation
- **CLAUDE.md**: This comprehensive specification
- **API Documentation**: Built into web UI at `/`

### Repository
- **GitHub**: https://github.com/apimgr/anime
- **Issues**: https://github.com/apimgr/anime/issues
- **Releases**: https://github.com/apimgr/anime/releases
- **Container Registry**: https://ghcr.io/apimgr/anime

### CI/CD
- **GitHub Actions**: Automated builds and releases
- **Jenkins**: https://jenkins.casjay.cc (optional)

### Development
- **Go Version**: 1.23 or later
- **Docker**: For containerization and testing
- **Incus**: Alternative to Docker for testing

---

## Appendix

### Environment Variables Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` (host), `80` (Docker) | Server port |
| `ADDRESS` | `::` | Bind address (:: = dual-stack, 0.0.0.0 = IPv4) |
| `DATA_DIR` | OS-specific | Data directory for database |
| `CONFIG_DIR` | OS-specific | Config directory for GeoIP |
| `LOGS_DIR` | OS-specific | Logs directory |
| `DEV` | `false` | Development mode |

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | Server port |
| `--address` | `::` | Bind address |
| `--data` | OS-specific | Data directory |
| `--version` | - | Print version and exit |
| `--status` | - | Health check (exit 0 if OK) |

### Rate Limiting Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `rate_limit_global_rps` | 100 | Global requests per second |
| `rate_limit_global_burst` | 200 | Global burst capacity |
| `rate_limit_api_rps` | 50 | API requests per second |
| `rate_limit_api_burst` | 100 | API burst capacity |
| `rate_limit_admin_rps` | 10 | Admin requests per second |
| `rate_limit_admin_burst` | 20 | Admin burst capacity |

### Security Headers Reference

| Header | Default Value |
|--------|---------------|
| X-Frame-Options | `DENY` |
| X-Content-Type-Options | `nosniff` |
| X-XSS-Protection | `1; mode=block` |
| Referrer-Policy | `strict-origin-when-cross-origin` |
| Permissions-Policy | `geolocation=(), microphone=(), camera=()` |
| Content-Security-Policy | `default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'` |
| Strict-Transport-Security | `max-age=31536000; includeSubDomains` (HTTPS only) |

### GeoIP Database Details

| Database | File | Size | Purpose |
|----------|------|------|---------|
| City IPv4 | `geolite2-city-ipv4.mmdb` | ~25MB | City-level IPv4 lookups |
| City IPv6 | `geolite2-city-ipv6.mmdb` | ~25MB | City-level IPv6 lookups |
| Country | `geo-whois-asn-country.mmdb` | ~25MB | Country lookups (IPv4/IPv6) |
| ASN | `asn.mmdb` | ~25MB | ASN and organization (IPv4/IPv6) |

**Total**: ~103MB, auto-downloaded on first run, updated weekly (Sunday 3:00 AM)

---

**Document End** - Last Updated: 2025-10-16
