# Anime Quotes API - Complete Project Specification

**Project**: Anime Quotes API
**Repository**: `github.com/apimgr/anime`
**Version**: 0.0.1
**Go Version**: 1.23
**Description**: Production-ready REST API serving 2000+ curated anime quotes in a 17MB static binary
**Status**: Active Development

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Critical Rules](#critical-rules)
3. [Technology Stack](#technology-stack)
4. [Architecture](#architecture)
5. [Directory Structure](#directory-structure)
6. [Features](#features)
7. [GeoIP Status](#geoip-status)
8. [API Endpoints](#api-endpoints)
9. [Build System](#build-system)
10. [Testing](#testing)
11. [Docker](#docker)
12. [CI/CD](#cicd)
13. [Security](#security)
14. [Admin Panel](#admin-panel)
15. [Database](#database)
16. [Frontend](#frontend)
17. [Deployment](#deployment)
18. [Version Control](#version-control)
19. [Development Workflow](#development-workflow)
20. [Production Checklist](#production-checklist)

---

## Project Overview

The Anime Quotes API is a lightweight, production-ready REST API that serves over 2000 curated anime quotes. It's built as a single 17MB static binary with all assets embedded, requiring no external dependencies or files to run.

### Key Characteristics

- **Static Data**: Serves pre-loaded quotes from embedded JSON (no dynamic data)
- **No Geographic Features**: Does not provide location-based services
- **Embedded Assets**: All HTML, CSS, JS, and JSON data built into binary
- **Self-Contained**: No external databases or services required at runtime
- **Production-Ready**: Full security, rate limiting, and admin features

---

## Critical Rules

### SPEC Compliance

**This project follows the complete SPEC.md** located at `/root/Projects/github/apimgr/SPEC.md` (9592 lines).

**Read SPEC.md Before:**

- Making any architectural changes
- Implementing new features
- Modifying build/deployment processes
- Changing security configurations

### AI Assistant Guidelines

**MUST Follow:**

1. **Build with Docker ONLY** - Never build on host OS

   ```bash
   # Correct
   make build
   docker run --rm -v $(pwd):/workspace golang:alpine ...

   # WRONG
   go build ./src
   ```

2. **Test in Containers** - Priority: Incus > Docker > Host OS

   ```bash
   # Preferred
   incus launch images:alpine/3.19 test-alpine

   # Alternative
   docker-compose -f docker-compose.test.yml up -d

   # FORBIDDEN (unless explicitly requested)
   ./binaries/anime --port 8080
   ```

3. **No Version Control** - AI NEVER executes git commands
   - ❌ `git add`, `git commit`, `git push`
   - ✅ `git status`, `git diff`, `git log` (read-only)

4. **Question vs Command Detection**
   - Ends with `?` = Question (Answer, don't execute)
   - Imperative verb = Command (Execute after confirmation if unclear)

### Build Requirements

**Static Binary (MANDATORY):**

```bash
CGO_ENABLED=0 go build -ldflags "-w -s" ./src
```

**Multi-Platform Builds:**

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64, arm64
- FreeBSD: amd64, arm64

**Total: 8 platform binaries per release**

---

## Technology Stack

### Backend

- **Language**: Go 1.23
- **Router**: Gorilla Mux
- **Database**: SQLite3 (for admin features/settings)
- **Rate Limiting**: go-chi/httprate
- **Authentication**: Basic Auth + Bearer tokens (bcrypt hashed)

### Frontend

- **Templates**: Go html/template (embedded)
- **CSS**: Vanilla CSS with custom properties (~867 lines)
- **JavaScript**: Vanilla JS (~130 lines)
- **Theme**: Dark mode default (light mode available)
- **No External Dependencies**: No Bootstrap, jQuery, React, etc.

### Infrastructure

- **Container Base**: golang:alpine (builder) + alpine:latest (runtime)
- **Orchestration**: Docker Compose
- **CI/CD**: GitHub Actions + Jenkins (jenkins.casjay.cc)
- **Documentation**: MkDocs with Dracula theme (ReadTheDocs)

---

## Architecture

### Single Binary Design

```
anime (17MB static binary)
├── Embedded JSON Data (src/data/anime.json)
├── Embedded Templates (src/server/templates/*)
├── Embedded CSS (src/server/static/css/main.css)
├── Embedded JS (src/server/static/js/main.js)
├── Embedded Images (favicon, logo)
└── SQLite Database (optional, for admin/settings)
```

**All assets are embedded via `go:embed` - binary is truly self-contained.**

### Component Architecture

```
┌─────────────────────────────────────────┐
│         HTTP Server (Gorilla Mux)       │
│  (Rate Limiting, Security Headers)      │
└──────────────┬──────────────────────────┘
               │
    ┌──────────┴──────────┐
    │                     │
┌───▼────────┐    ┌──────▼──────────┐
│   Public   │    │  Admin Panel    │
│   API      │    │  (Authenticated) │
└───┬────────┘    └──────┬──────────┘
    │                    │
    │              ┌─────▼──────┐
    │              │  Settings  │
    │              │  Manager   │
    │              └────────────┘
    │
┌───▼──────────────┐
│  Anime Service   │
│  (Embedded JSON) │
└──────────────────┘
```

### Data Flow

1. **Startup**: Binary loads embedded JSON data into memory
2. **Request**: HTTP request → Router → Rate Limiter → Handler
3. **Processing**: Handler queries in-memory data structure
4. **Response**: JSON response returned to client
5. **Admin**: Settings stored in SQLite, applied via live reload

---

## Directory Structure

### Repository Layout

```
/root/Projects/github/apimgr/anime/
├── .github/
│   └── workflows/
│       ├── release.yml            # Binary builds + GitHub releases
│       └── docker.yml             # Docker multi-arch builds
├── .gitignore                     # Git ignore patterns
├── CLAUDE.md                      # This file (project specification)
├── Dockerfile                     # Alpine-based multi-stage build
├── docker-compose.yml             # Production deployment
├── docker-compose.test.yml        # Development/testing
├── go.mod                         # Go module definition
├── go.sum                         # Go module checksums
├── Jenkinsfile                    # CI/CD pipeline (jenkins.casjay.cc)
├── LICENSE.md                     # MIT License
├── Makefile                       # Build system (4 targets)
├── README.md                      # User documentation
├── release.txt                    # Version: 0.0.1 (no "v" prefix)
├── binaries/                      # Built binaries (gitignored)
│   ├── anime-linux-amd64
│   ├── anime-linux-arm64
│   ├── anime-windows-amd64.exe
│   ├── anime-windows-arm64.exe
│   ├── anime-macos-amd64
│   ├── anime-macos-arm64
│   ├── anime-bsd-amd64
│   └── anime-bsd-arm64
├── releases/                      # Release artifacts (gitignored)
│   └── anime-{VERSION}-src.tar.gz # Source archives for GitHub releases
├── rootfs/                        # Docker volumes (gitignored)
│   ├── config/anime/
│   ├── data/anime/
│   └── logs/anime/
└── src/                           # Source code
    ├── data/
    │   └── anime.json             # 2000+ quotes (JSON ONLY, no .go files)
    ├── anime/
    │   └── service.go             # Quote service (loads from embedded JSON)
    ├── database/
    │   ├── database.go            # SQLite connection & schema
    │   ├── auth.go                # Admin authentication
    │   ├── credentials.go         # Credential management
    │   └── settings.go            # Settings CRUD
    ├── paths/
    │   └── paths.go               # OS-specific directory detection
    ├── server/
    │   ├── server.go              # Server setup & routing
    │   ├── handlers.go            # Public API handlers
    │   ├── admin_handlers.go      # Admin panel handlers
    │   ├── auth_middleware.go     # Authentication middleware
    │   ├── middleware.go          # Security & rate limiting
    │   ├── settings_manager.go    # Live settings management
    │   ├── templates.go           # Template embedding
    │   ├── static/                # Static assets (embedded)
    │   │   ├── css/
    │   │   │   └── main.css       # Dark theme CSS (~867 lines)
    │   │   ├── js/
    │   │   │   └── main.js        # Vanilla JS (~130 lines)
    │   │   ├── images/
    │   │   │   └── favicon.png
    │   │   └── manifest.json      # PWA manifest
    │   └── templates/             # HTML templates (embedded)
    │       ├── base.html          # Base layout
    │       ├── home.html          # Homepage
    │       ├── admin.html         # Admin dashboard
    │       └── admin_settings.html # Settings page
    └── main.go                    # Entry point (embeds data/anime.json)
```

### Key Points

- **`src/data/`**: Contains ONLY JSON files (no .go code)
- **Embedding**: `main.go` embeds `data/anime.json` via `//go:embed`
- **Assets**: All templates, CSS, JS embedded in binary
- **Database**: SQLite stored in `{DATA_DIR}/db/anime.db`
- **Releases**: Binaries go to `releases/` not `release/` (per SPEC)

---

## Features

### Core Features (Implemented)

✅ **Quote API**

- Random quote endpoint
- Get all quotes endpoint
- In-memory data structure (fast lookups)
- 2000+ curated quotes from various anime

✅ **Web Interface**

- Dark theme by default (light mode toggle)
- Responsive design (mobile-friendly)
- Homepage with quote display
- Admin panel UI

✅ **Admin Panel**

- Basic authentication (username/password)
- Settings management
- Live configuration reload (no restart needed)
- Admin credentials stored in SQLite

✅ **Security**

- Rate limiting (100 global, 50 API, 10 admin req/s)
- Security headers (X-Frame-Options, CSP, etc.)
- CORS configurable via admin panel (default: allow all)
- bcrypt password hashing
- Bearer token authentication

✅ **Infrastructure**

- Docker support (multi-arch: amd64, arm64)
- Docker Compose (production + test configs)
- Makefile build system
- GitHub Actions CI/CD
- Jenkins pipeline (jenkins.casjay.cc)

### Features Per SPEC (Status)

✅ **Mandatory (Implemented)**

- Dockerfile (golang:alpine + alpine:latest)
- docker-compose.yml & docker-compose.test.yml
- Makefile (build, test, release, docker targets)
- GitHub Actions (release.yml + docker.yml)
- Jenkinsfile (multi-arch pipeline)
- src/data/ directory (JSON only, embedded)
- Web UI with dark theme
- Admin panel with live reload
- Rate limiting & DDoS protection
- Security headers
- Database in {DATA_DIR}/db/
- CORS default: `*` (configurable)
- All 8 platform binaries
- Version: 0.0.1 (no "v" prefix)

⏳ **Mandatory (To Be Implemented)**

- ReadTheDocs (MkDocs + Dracula theme)
- Swagger/OpenAPI documentation
- GraphQL endpoint
- Multi-distro testing (Incus)
- Install scripts (all platforms)
- Source archives in releases
- Full admin settings UI
- Scheduler (for periodic tasks if needed)
- Signal handling (SIGTERM, SIGHUP, etc.)
- Service management commands
- Logging system (access.log, error.log, audit.log)

❌ **Not Required (Per SPEC Exception Rules)**

- GeoIP integration (no location-based features)

---

## GeoIP Status

### Why GeoIP is NOT Included

**Per SPEC Section 16 (Mandatory vs Optional Features):**

GeoIP is **ONLY required** for projects with location-based features:

- "Find nearby X" functionality
- "Where am I?" features
- Geographic search/filtering
- IP-based country/region detection

### Anime Quotes API Analysis

❌ **Does NOT need GeoIP because:**

- Serves static anime quotes (no location context)
- No "find nearby" functionality
- No geographic search or filtering
- No IP-based personalization
- Pure content API (quotes are global/universal)

✅ **Correctly excluded per SPEC exception rules**

### SPEC Reference

```
Exception 2: GeoIP Integration - LOCATION-BASED PROJECTS ONLY
- ✅ Required IF: Project has location-based features
- ❌ Not required IF: No geographic features
  - Static data APIs ← ANIME QUOTES FALLS HERE
  - No location context needed

Examples:
- airports: ✅ GeoIP (find airports near IP)
- zipcodes: ✅ GeoIP (find ZIP codes near IP)
- citylist: ❌ No GeoIP (cities don't need IP location)
- anime: ❌ No GeoIP (quotes are static, global)
```

**Conclusion**: GeoIP is correctly excluded from anime quotes API per SPEC guidelines.

---

## API Endpoints

### Public API (No Authentication)

#### Get Random Quote

```http
GET /api/v1/random
```

**Response:**

```json
{
  "success": true,
  "data": {
    "anime": "Fullmetal Alchemist",
    "character": "Edward Elric",
    "quote": "A lesson without pain is meaningless."
  }
}
```

#### Get All Quotes

```http
GET /api/v1/quotes
```

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "anime": "Bleach",
      "character": "Ichigo Kurosaki",
      "quote": "I'm not a hero. I'm just a guy who's good at fighting."
    },
    ...
  ],
  "meta": {
    "total": 2134
  }
}
```

#### Health Check

```http
GET /api/v1/health
```

**Response:**

```json
{
  "status": "healthy",
  "timestamp": "2025-10-16T12:00:00Z",
  "version": "0.0.1",
  "totalQuotes": 2134,
  "uptime": "24h15m30s"
}
```

### Admin API (Requires Authentication)

#### Get Settings

```http
GET /api/v1/admin/settings
Authorization: Bearer <token>
```

#### Update Settings

```http
PUT /api/v1/admin/settings
Authorization: Bearer <token>
Content-Type: application/json

{
  "settings": {
    "server.cors_origins": ["https://example.com"],
    "rate.global_rps": 200
  }
}
```

### Web UI Routes

- `/` - Homepage (random quote display)
- `/admin` - Admin dashboard (requires auth)
- `/admin/settings` - Settings configuration page
- `/static/*` - Static assets (CSS, JS, images)

---

## Build System

### Makefile Targets

```bash
# Build binaries for all 8 platforms
make build

# Run tests with coverage
make test

# Clean build artifacts
make clean

# Build and push Docker images (multi-arch)
make docker

# Build Docker image for local development
make docker-dev

# Test with Docker Compose
make test-docker

# Test with Incus (alternative)
make test-incus

# Full CI pipeline (test + build)
make all
```

### Build Details

**Build Command (Docker):**

```bash
docker run --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  golang:alpine \
  sh -c 'CGO_ENABLED=0 go build -ldflags "-w -s -X main.Version=0.0.1" -o binaries/anime-linux-amd64 ./src'
```

**Platforms Built:**

1. linux-amd64
2. linux-arm64
3. windows-amd64.exe
4. windows-arm64.exe
5. macos-amd64
6. macos-arm64
7. freebsd-amd64
8. freebsd-arm64

**Binary Size:** ~17MB static (all assets embedded)

### Version Management

**Location:** `release.txt`
**Format:** `0.0.1` (NO "v" prefix)
**Update:** Manual (not automated)

```bash
# Current version
cat release.txt
# Output: 0.0.1

# To release new version
echo "0.0.2" > release.txt
make build
```

---

## Testing

### Testing Priority (Per SPEC)

**For Building:**

1. ✅ Docker ONLY (golang:alpine)

**For Testing/Debugging:**

1. 🥇 Incus (preferred) - System containers
2. 🥈 Docker (fallback) - If Incus unavailable
3. 🥉 Host OS (last resort) - Only when explicitly requested

### Multi-Distro Testing (Required)

**Test on multiple distributions (Incus):**

1. **Alpine** (musl libc, no systemd)

```bash
incus launch images:alpine/3.19 test-alpine
incus file push ./binaries/anime test-alpine/usr/local/bin/
incus exec test-alpine -- /usr/local/bin/anime --version
incus delete -f test-alpine
```

2. **Ubuntu 24.04** (glibc, systemd) - MANDATORY

```bash
incus launch images:ubuntu/24.04 test-ubuntu
# Test systemd service installation
incus file push scripts/install-linux.sh test-ubuntu/tmp/
incus exec test-ubuntu -- bash /tmp/install-linux.sh
incus exec test-ubuntu -- systemctl status anime
incus delete -f test-ubuntu
```

3. **Fedora** (SELinux, dnf) - Optional but recommended

```bash
incus launch images:fedora/40 test-fedora
incus file push ./binaries/anime test-fedora/usr/local/bin/
incus exec test-fedora -- /usr/local/bin/anime --version
incus delete -f test-fedora
```

### Port Selection

**ALWAYS use random ports (64000-64999):**

```bash
TESTPORT=$(shuf -i 64000-64999 -n 1)
./binaries/anime --port $TESTPORT
```

**NEVER use:** 80, 443, 8080, 3000, 5000, or any common ports

### Test Data Location

**ALWAYS use /tmp/ for test data:**

```bash
# Correct
./binaries/anime \
  --data /tmp/anime-test/data \
  --port 64555

# WRONG
./binaries/anime \
  --data /var/lib/anime \
  --port 8080
```

---

## Docker

### Dockerfile

**Multi-stage build:**

```dockerfile
# Build stage
FROM golang:alpine AS builder
# ... build static binary ...

# Runtime stage
FROM alpine:latest
# Install: curl, bash, ca-certificates, tzdata
# Copy binary to /usr/local/bin/anime
# Run as nobody (65534:65534)
```

**Key Points:**

- Builder: `golang:alpine` (latest)
- Runtime: `alpine:latest` (not scratch)
- Binary location: `/usr/local/bin/anime`
- User: nobody (UID 65534)
- Health check: `/usr/local/bin/anime --status`

### Docker Compose

**Production** (`docker-compose.yml`):

```yaml
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
    networks:
      - anime
```

**Development** (`docker-compose.test.yml`):

```yaml
services:
  anime:
    image: anime:dev
    container_name: anime-test
    restart: "no"
    ports:
      - "64181:80"
    volumes:
      - /tmp/anime/rootfs/config/anime:/config
      - /tmp/anime/rootfs/data/anime:/data
      - /tmp/anime/rootfs/logs/anime:/logs
```

**Key Differences:**

- Production: pre-built image, persistent storage, Docker bridge IP
- Development: local build, ephemeral /tmp storage, localhost

### Volume Structure

```
./rootfs/
├── config/anime/          # Configuration files
│   ├── admin_credentials  # Admin username/password/token
│   └── settings.json      # Server settings (if file-based)
├── data/anime/            # Application data
│   └── db/
│       └── anime.db       # SQLite database
└── logs/anime/            # Log files
    ├── access.log
    ├── error.log
    └── anime.log
```

---

## CI/CD

### GitHub Actions

**Location:** `.github/workflows/`

#### release.yml (Binary Builds)

- **Trigger:** Push to main, monthly (1st @ 3AM UTC), manual
- **Jobs:**
  1. Test (Go 1.23)
  2. Build (all 8 platforms)
  3. Create GitHub release with binaries

#### docker.yml (Docker Images)

- **Trigger:** Push to main, monthly (1st @ 3AM UTC), manual
- **Jobs:**
  1. Build multi-arch images (amd64, arm64)
  2. Push to `ghcr.io/apimgr/anime`
  3. Tags: `latest`, `0.0.1`, `main-{sha}`

**Key Points:**

- Reads version from `release.txt` (does NOT modify it)
- Deletes existing release if exists (always recreate)
- Multi-arch support via buildx
- GitHub cache for faster builds

### Jenkins Pipeline

**Server:** jenkins.casjay.cc
**Location:** `Jenkinsfile`

**Pipeline:**

```groovy
1. Checkout (amd64 agent)
2. Test (parallel: amd64, arm64)
3. Build Binaries (parallel: amd64, arm64)
4. Build Docker Images (parallel: amd64, arm64)
5. Push Docker Images (manifest creation)
6. GitHub Release (if main/master branch)
```

**Key Features:**

- Multi-agent pipeline (amd64, arm64)
- Parallel builds and tests
- Docker manifest creation for multi-arch
- Automatic cleanup (cleanWs)

---

## Security

### Rate Limiting (Implemented)

**Configuration (configurable via admin panel):**

```go
Global:  100 req/s (burst: 200)
API:     50 req/s (burst: 100)
Admin:   10 req/s (burst: 20)
```

**Implementation:** go-chi/httprate middleware

**Response on limit:**

```http
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1234567890
Retry-After: 60

{
  "success": false,
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests. Please try again later.",
    "retry_after": 60
  }
}
```

### Security Headers (Implemented)

```http
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

**HSTS (if HTTPS):**

```http
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

### CORS (Configurable)

**Default:** Allow all origins (`*`)
**Configurable via:** Admin panel (`/admin/settings`)

```json
{
  "server.cors_enabled": true,
  "server.cors_origins": ["*"],
  "server.cors_methods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
  "server.cors_headers": ["Content-Type", "Authorization"]
}
```

**Production Recommendation:**

```json
{
  "server.cors_origins": ["https://yourdomain.com", "https://app.yourdomain.com"]
}
```

### Authentication

**Admin Panel:**

- Basic Auth (username/password)
- Bearer tokens (API access)
- bcrypt password hashing (cost: 10)
- Token length: 64 characters (cryptographically random)

**Brute Force Protection:**

- Track failed login attempts per IP
- Block after 5 failed attempts
- Reset on successful login

**Credentials Storage:**

- File: `{CONFIG_DIR}/admin_credentials`
- Permissions: 0600 (read/write owner only)
- Format: Plain text (for first-time setup), bcrypt in database

---

## Admin Panel

### Features

✅ **Dashboard** (`/admin`)

- Server statistics
- Quote count
- System health
- Recent activity

✅ **Settings Management** (`/admin/settings`)

- CORS configuration
- Rate limiting
- Security headers
- Request limits
- Live reload (no restart needed)

⏳ **To Be Implemented**

- Log viewer
- Database status
- System metrics
- Configuration export/import

### Settings Categories

| Category   | Settings                  | Live Reload |
| ---------- | ------------------------- | ----------- |
| `server`   | CORS, title, description  | ✅ Yes       |
| `rate`     | All rate limiting configs | ✅ Yes       |
| `request`  | Timeouts, size limits     | ✅ Yes       |
| `security` | Headers, policies         | ✅ Yes       |
| `log`      | Logging levels, formats   | ✅ Yes       |

### Live Reload Mechanism

**How it works:**

1. Admin updates setting via WebUI
2. Setting saved to SQLite database
3. SettingsManager detects change (channel notification)
4. Server reloads settings from database
5. New settings applied immediately (no restart)

**Thread-safe:** Uses `sync.RWMutex` for concurrent access

---

## Database

### Schema

**SQLite database location:** `{DATA_DIR}/db/anime.db`

#### Tables

**1. admin_users**

```sql
CREATE TABLE admin_users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  token TEXT UNIQUE NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**2. settings**

```sql
CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('string', 'number', 'boolean', 'json')),
  category TEXT NOT NULL,
  description TEXT,
  requires_reload BOOLEAN DEFAULT 0,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Default Settings

```sql
INSERT INTO settings (key, value, type, category, description) VALUES
  -- CORS
  ('server.cors_enabled', 'true', 'boolean', 'server', 'Enable CORS'),
  ('server.cors_origins', '["*"]', 'json', 'server', 'Allowed origins'),
  ('server.cors_methods', '["GET","POST","PUT","DELETE","OPTIONS"]', 'json', 'server', 'Allowed methods'),

  -- Rate Limiting
  ('rate.enabled', 'true', 'boolean', 'rate', 'Enable rate limiting'),
  ('rate.global_rps', '100', 'number', 'rate', 'Global requests per second'),
  ('rate.api_rps', '50', 'number', 'rate', 'API requests per second'),
  ('rate.admin_rps', '10', 'number', 'rate', 'Admin requests per second'),

  -- Security
  ('security.frame_options', 'DENY', 'string', 'security', 'X-Frame-Options'),
  ('security.csp', 'default-src ''self''', 'string', 'security', 'Content-Security-Policy');
```

### Connection Management

**Connection pooling:**

```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

**Health check:**

```go
if err := db.Ping(); err != nil {
    // Enter maintenance mode
}
```

---

## Frontend

### Technology

- **Templates:** Go html/template (server-side rendering)
- **CSS:** Vanilla CSS with custom properties (~867 lines)
- **JavaScript:** Vanilla JS (~130 lines)
- **No Frameworks:** No Bootstrap, Tailwind, React, jQuery, etc.

### Theme

**Dark mode by default:**

```css
:root {
  --bg-primary: #1a1a1a;
  --bg-secondary: #2d2d2d;
  --bg-tertiary: #404040;
  --text-primary: #ffffff;
  --text-secondary: #b0b0b0;
  --accent-primary: #0066cc;
}
```

**Light mode (toggle):**

```css
[data-theme="light"] {
  --bg-primary: #ffffff;
  --bg-secondary: #f5f5f5;
  --text-primary: #1a1a1a;
  --border-color: #e0e0e0;
}
```

### Components

**Buttons, Cards, Modals, Toasts, Forms, Grids:**

- All styled with CSS custom properties
- Smooth transitions (150ms, 300ms)
- Hover effects and shadows
- Mobile-responsive

### Responsive Design

**Desktop (≥720px):**

- Content width: 90%
- Max-width: 1200px (optional)
- Left/right margins: 5%

**Mobile (<720px):**

- Content width: 98%
- Left/right margins: 1%
- Hamburger menu for navigation

### Accessibility

✅ **Features:**

- Semantic HTML5 tags
- ARIA labels
- Keyboard navigation
- Proper contrast ratios
- Alt text for images
- Logical tab order

---

## Deployment

### Binary Installation

**Download from GitHub Releases:**

```bash
# Linux (amd64)
wget https://github.com/apimgr/anime/releases/download/0.0.1/anime-linux-amd64
chmod +x anime-linux-amd64
./anime-linux-amd64 --port 8080
```

**Platforms Available:**

- Linux: amd64, arm64
- Windows: amd64, arm64
- macOS: amd64, arm64
- FreeBSD: amd64, arm64

### OS-Specific Directories

**Linux (root):**

```
Config:  /etc/anime/
Data:    /var/lib/anime/
Logs:    /var/log/anime/
```

**Linux (user):**

```
Config:  ~/.config/anime/
Data:    ~/.local/share/anime/
Logs:    ~/.local/state/anime/
```

**macOS (root):**

```
Config:  /Library/Application Support/Anime/
Data:    /Library/Application Support/Anime/data/
Logs:    /Library/Logs/Anime/
```

**macOS (user):**

```
Config:  ~/Library/Application Support/Anime/
Data:    ~/Library/Application Support/Anime/data/
Logs:    ~/Library/Logs/Anime/
```

**Windows (admin):**

```
Config:  C:\ProgramData\Anime\config\
Data:    C:\ProgramData\Anime\data\
Logs:    C:\ProgramData\Anime\logs\
```

**Docker:**

```
Config:  /config
Data:    /data
Logs:    /logs
```

### Environment Variables

```bash
# Server configuration
PORT=8080                  # Server port
ADDRESS=::                 # Listen address (:: = dual-stack IPv4+IPv6)

# Directories
CONFIG_DIR=/etc/anime
DATA_DIR=/var/lib/anime
LOGS_DIR=/var/log/anime

# Admin credentials (first-time setup)
ADMIN_USER=administrator
ADMIN_PASSWORD=changeme
ADMIN_TOKEN=your-64-char-token-here

# Database
DB_PATH=/var/lib/anime/db/anime.db

# Debug mode
DEBUG=1                    # Enable debug mode (NEVER in production)
```

### Docker Deployment

**Production:**

```bash
docker-compose up -d
# Access at http://172.17.0.1:64180
```

**Development:**

```bash
make docker-dev
docker-compose -f docker-compose.test.yml up -d
# Access at http://localhost:64181
```

---

## Version Control

### Git Workflow

**AI NEVER executes git commands:**

- ❌ `git add`, `git commit`, `git push`, `git tag`
- ❌ Any write operations
- ✅ `git status`, `git diff`, `git log` (read-only)

**After making changes, AI reports:**

```
Changes Complete:

Files Modified:
1. src/main.go - Added IPv6 support
2. README.md - Updated documentation
3. Dockerfile - Changed to alpine:latest

Testing:
- Build verified: ✓
- Binary size: 17MB
- Version output: 0.0.1

Next Steps:
- Review changes: git diff
- When ready: git add . && git commit -m "..." && git push
```

### Release Process

1. **Update version:** `echo "0.0.2" > release.txt`
2. **Build:** `make build`
3. **Test:** `make test`
4. **Commit:** `git add . && git commit -m "Release 0.0.2"`
5. **Push:** `git push`
6. **GitHub Actions automatically:**
   - Builds all 8 platform binaries
   - Creates GitHub release `0.0.2`
   - Uploads binaries
   - Builds and pushes Docker images

---

## Development Workflow

### Local Development

```bash
# Clone repository
git clone https://github.com/apimgr/anime.git
cd anime

# Download dependencies
go mod download

# Build development binary
make build

# Run locally
./binaries/anime --port 8080

# Or with Docker
make docker-dev
docker-compose -f docker-compose.test.yml up
```

### Making Changes

**Before changing code:**

1. Read SPEC.md for standards
2. Check existing patterns in codebase
3. Understand affected components

**After changing code:**

1. Build: `make build`
2. Test: `make test`
3. Verify binary: `./binaries/anime --version`
4. Test in Docker: `make test-docker`
5. Multi-distro test: `make test-incus`

### Testing Checklist

- [ ] Code compiles (CGO_ENABLED=0)
- [ ] All tests pass (`make test`)
- [ ] Binary runs on Alpine (musl test)
- [ ] Binary runs on Ubuntu (glibc test)
- [ ] Docker image builds successfully
- [ ] docker-compose.test.yml works
- [ ] Health check passes (`--status`)
- [ ] Admin panel accessible
- [ ] API endpoints return correct responses

---

## Production Checklist

### Pre-Deployment

- [ ] Version updated in `release.txt`
- [ ] All tests passing
- [ ] Multi-distro testing complete
- [ ] Security headers enabled
- [ ] Rate limiting configured
- [ ] CORS properly restricted
- [ ] Admin credentials changed
- [ ] Database backups configured
- [ ] Logs properly configured
- [ ] Monitoring setup (if applicable)

### Deployment

- [ ] Binary uploaded to server
- [ ] Directories created (config, data, logs)
- [ ] Permissions set correctly
- [ ] Service installed (systemd/launchd/etc.)
- [ ] Service starts automatically
- [ ] Health check passes
- [ ] API accessible
- [ ] Admin panel accessible
- [ ] SSL/TLS configured (if needed)
- [ ] Reverse proxy configured (nginx/Caddy)

### Post-Deployment

- [ ] Monitor logs for errors
- [ ] Check memory usage
- [ ] Verify rate limiting working
- [ ] Test all API endpoints
- [ ] Backup database
- [ ] Document any issues
- [ ] Update documentation if needed

---

## Key Differences from SPEC

### Mandatory Features (Completed)

- ✅ Docker (golang:alpine + alpine:latest)
- ✅ docker-compose.yml & test.yml
- ✅ Makefile (4 targets)
- ✅ GitHub Actions (2 workflows)
- ✅ Jenkinsfile
- ✅ src/data/ (JSON only)
- ✅ Web UI (dark theme)
- ✅ Admin panel (basic)
- ✅ Rate limiting
- ✅ Security headers
- ✅ Database (SQLite)
- ✅ CORS (default: *)
- ✅ Version: 0.0.1 (no "v")

### Mandatory Features (Pending)

- ⏳ ReadTheDocs (MkDocs + Dracula)
- ⏳ OpenAPI/Swagger docs
- ⏳ GraphQL endpoint
- ⏳ Multi-distro testing
- ⏳ Install scripts
- ⏳ Source archives
- ⏳ Full settings UI
- ⏳ Scheduler
- ⏳ Signal handling
- ⏳ Logging system

### Optional Features (Status)

- ✅ IPv6 Support: Implemented with auto-detect fallback (default: `::`)
- ❌ GeoIP: Correctly excluded (no location features)

---

## Conclusion

The Anime Quotes API is a production-ready REST API following the complete SPEC.md standards. It serves 2000+ quotes in a 17MB self-contained binary with all assets embedded. The project correctly excludes GeoIP per SPEC exception rules (no location-based features) and implements all mandatory infrastructure components.

**Current Status:** Active development, version 0.0.1, ready for production deployment with remaining SPEC features to be implemented.

**SPEC Compliance:** ~70% complete (all core features done, documentation/testing/tooling pending)

---

**Last Updated:** 2025-10-16
**SPEC Version:** 2.0 (9592 lines)
**Project Version:** 0.0.1
