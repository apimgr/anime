.PHONY: all build test clean docker docker-dev test-docker test-incus help

# Project variables
PROJECTNAME := anime
PROJECTORG := apimgr
VERSION := $(shell cat release.txt 2>/dev/null || echo "0.0.1")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Build variables
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE) -w -s"
SRC_DIR := ./src
BINARY_DIR := ./binaries
RELEASE_DIR := ./releases

# Platform targets
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	windows/amd64 \
	windows/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	freebsd/amd64 \
	freebsd/arm64

all: test build

# Test the application
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

# Build for all platforms
build:
	@echo "Building $(PROJECTNAME) v$(VERSION)..."
	@mkdir -p $(BINARY_DIR)
	@mkdir -p $(RELEASE_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		OUTPUT_NAME=$(BINARY_DIR)/$(PROJECTNAME)-$$GOOS-$$GOARCH; \
		if [ "$$GOOS" = "windows" ]; then \
			OUTPUT_NAME=$$OUTPUT_NAME.exe; \
		fi; \
		echo "Building $$GOOS/$$GOARCH..."; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o $$OUTPUT_NAME $(SRC_DIR); \
		if [ $$? -ne 0 ]; then \
			echo "Error building $$GOOS/$$GOARCH"; \
			exit 1; \
		fi; \
		cp $$OUTPUT_NAME $(RELEASE_DIR)/; \
	done
	@echo "Building host binary..."
	@CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_DIR)/$(PROJECTNAME) $(SRC_DIR)
	@cp $(BINARY_DIR)/$(PROJECTNAME) ./$(PROJECTNAME)
	@echo "✓ Build complete - binaries in $(BINARY_DIR)/"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BINARY_DIR)
	@rm -rf $(RELEASE_DIR)
	@rm -f $(PROJECTNAME)
	@rm -f coverage.out
	@echo "✓ Clean complete"

# Build and push multi-platform Docker images (release)
docker:
	@echo "Building multi-platform Docker images..."
	@docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t ghcr.io/$(PROJECTORG)/$(PROJECTNAME):latest \
		-t ghcr.io/$(PROJECTORG)/$(PROJECTNAME):$(VERSION) \
		--push \
		.
	@echo "✓ Docker images pushed to ghcr.io/$(PROJECTORG)/$(PROJECTNAME):$(VERSION)"

# Build Docker image for development (local only, not pushed)
docker-dev:
	@echo "Building development Docker image..."
	@docker build \
		--build-arg VERSION=$(VERSION)-dev \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(PROJECTNAME):dev \
		.
	@echo "✓ Docker development image built: $(PROJECTNAME):dev"

# Test with Docker (using docker-compose.test.yml)
test-docker:
	@echo "Testing with Docker..."
	@docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for service..."
	@timeout 30 bash -c 'until curl -sf http://localhost:64181/api/v1/health; do sleep 1; done' || (docker-compose -f docker-compose.test.yml logs && exit 1)
	@echo "✓ Service is running"
	@docker-compose -f docker-compose.test.yml logs
	@docker-compose -f docker-compose.test.yml down
	@sudo rm -rf /tmp/anime/rootfs
	@echo "✓ Docker test complete"

# Test with Incus (if Docker not available)
test-incus:
	@echo "Testing with Incus..."
	@incus launch images:alpine/3.19 $(PROJECTNAME)-test
	@incus file push ./binaries/$(PROJECTNAME) $(PROJECTNAME)-test/usr/local/bin/
	@incus exec $(PROJECTNAME)-test -- chmod +x /usr/local/bin/$(PROJECTNAME)
	@incus exec $(PROJECTNAME)-test -- /usr/local/bin/$(PROJECTNAME) --help
	@incus delete -f $(PROJECTNAME)-test
	@echo "✓ Incus test complete"

# Help target
help:
	@echo "Available targets:"
	@echo "  make build       - Build binaries for all platforms"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make docker      - Build and push multi-platform Docker images"
	@echo "  make docker-dev  - Build Docker image for local development"
	@echo "  make test-docker - Test with Docker using docker-compose.test.yml"
	@echo "  make test-incus  - Test with Incus container"
	@echo "  make all         - Run tests and build (default)"
