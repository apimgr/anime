# Anime Quotes API

A simple, fast, and lightweight REST API for anime quotes. Built with Go, providing random anime quotes from a curated collection.

## About

- Fast and lightweight API built with Go
- Random anime quote endpoint
- Get all quotes endpoint
- Health check endpoint
- Docker support with multi-architecture images
- No external dependencies required

## API Endpoints

### Get Random Quote
```bash
GET /api/v1/random
```

Returns a random anime quote.

**Response:**
```json
{
  "anime": "Fullmetal Alchemist",
  "character": "Edward Elric",
  "quote": "A lesson without pain is meaningless."
}
```

### Get All Quotes
```bash
GET /api/v1/quotes
```

Returns all anime quotes in the database.

**Response:**
```json
[
  {
    "anime": "Bleach",
    "character": "Ichigo Kurosaki",
    "quote": "I'm not a hero. I'm just a guy who's good at fighting."
  },
  ...
]
```

### Health Check
```bash
GET /api/v1/health
```

Returns the service health status.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-10-14T16:00:00Z",
  "totalQuotes": 5000
}
```

## Production Installation

### Binary Installation

1. Download the latest binary for your platform from the [releases page](https://github.com/apimgr/anime/releases)

2. Make it executable (Linux/macOS):
```bash
chmod +x anime-linux-amd64
```

3. Run the server:
```bash
./anime-linux-amd64 --port 8080
```

### Environment Variables

- `PORT` - Server port (default: 8080)
- `ADDRESS` - Server bind address (default: 0.0.0.0)

## Docker Deployment

### Using Docker Compose (Production)

```bash
docker-compose up -d
```

The API will be available at `http://172.17.0.1:64180`

### Using Docker Compose (Development)

```bash
# Build dev image first
make docker-dev

# Run with test compose
docker-compose -f docker-compose.test.yml up
```

The API will be available at `http://localhost:64181`

### Using Docker Run

```bash
docker run -d \
  -p 8080:80 \
  --name anime \
  ghcr.io/apimgr/anime:latest
```

## Development

### Requirements

- Go 1.23 or later
- Make (optional, for build automation)
- Docker (optional, for containerization)

### Build System & Testing

**Makefile Targets:**
- `make build` - Build binaries for all 8 platforms (Linux, Windows, macOS, FreeBSD - amd64/arm64)
- `make test` - Run tests with coverage
- `make clean` - Clean build artifacts
- `make docker` - Build and push multi-platform Docker images to ghcr.io
- `make docker-dev` - Build Docker image for local development
- `make test-docker` - Test with Docker using docker-compose.test.yml
- `make test-incus` - Test with Incus container (alternative to Docker)

**Platforms Supported:**
- linux-amd64, linux-arm64
- windows-amd64, windows-arm64
- macos-amd64, macos-arm64
- freebsd-amd64, freebsd-arm64

**Version Management:**
- Version stored in `release.txt` (format: `0.0.1` without "v" prefix)
- Build: `make build` reads version, doesn't modify it
- Release: Manual update of `release.txt` when ready for new version

### Development Mode

Run with development flags:
```bash
./anime --dev --port 8080
```

### CI/CD

**GitHub Actions:**
- Automated builds on push to main
- Monthly scheduled builds (1st of month)
- Multi-platform binary releases
- Docker images pushed to ghcr.io/apimgr/anime

**Jenkins:**
- Multi-architecture pipeline (amd64, arm64)
- Server: jenkins.casjay.cc
- Parallel builds and tests
- Docker manifest creation for multi-arch support

### Build from Source

```bash
# Clone the repository
git clone https://github.com/apimgr/anime.git
cd anime

# Download dependencies
go mod download

# Build binary
make build

# Or build manually
go build -o anime ./src
```

### Run Locally

```bash
# Using make
make build
./anime --port 8080

# Or directly with go run
go run ./src --port 8080
```

### Testing

```bash
# Run tests
make test

# Or manually
go test -v ./...
```

### Build System

Available make targets:

- `make build` - Build binaries for all platforms
- `make test` - Run tests with coverage
- `make clean` - Clean build artifacts
- `make docker` - Build and push multi-platform Docker images
- `make docker-dev` - Build Docker image for local development
- `make all` - Run tests and build (default)

## Project Structure

```
anime/
├── src/
│   ├── data/
│   │   └── anime.json       # Quote database
│   ├── anime/
│   │   └── service.go       # Quote service logic
│   ├── server/
│   │   └── server.go        # HTTP server and handlers
│   └── main.go              # Application entry point
├── Dockerfile               # Container definition
├── docker-compose.yml       # Production compose
├── docker-compose.test.yml  # Development compose
├── Makefile                 # Build automation
├── go.mod                   # Go dependencies
└── README.md                # This file
```

## Example Usage

```bash
# Get a random quote
curl http://localhost:8080/api/v1/random

# Get all quotes
curl http://localhost:8080/api/v1/quotes

# Health check
curl http://localhost:8080/api/v1/health
```

## License

MIT License - See [LICENSE.md](LICENSE.md) for details

## Credits

Anime quotes sourced from various anime series and compiled for educational and entertainment purposes.
