# Infer the project identity from its Git remote, then fall back to its directory.
PROJECT_NAME := $(shell remote=$$(git remote get-url origin 2>/dev/null); if [ -n "$$remote" ]; then basename "$$remote" .git; else basename "$(PWD)"; fi)
PROJECT_ORG := $(shell remote=$$(git remote get-url origin 2>/dev/null); if [ -n "$$remote" ]; then printf "%s\n" "$$remote" | sed -E 's|.*/([^/]+)/[^/]+(\.git)?$$|\1|'; else basename "$(dir $(PWD))"; fi)
INTERNAL_NAME := anime
VERSION := $(shell cat release.txt 2>/dev/null || echo "$${VERSION:-devel}")
COMMIT_ID := $(shell git rev-parse --short=7 HEAD 2>/dev/null || echo unknown)
BUILD_EPOCH := $(shell date -u +%s)
OFFICIAL_SITE ?= $(shell cat site.txt 2>/dev/null)
BINDIR := binaries
RELDIR := releases
PLATFORMS ?= linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64,freebsd/amd64,freebsd/arm64
GO_CACHE ?= $(HOME)/go/pkg/mod
GO_BUILD ?= $(HOME)/.cache/go-build/$(PROJECT_NAME)
DOCKER_MEM ?= 4g
DOCKER_CPUS ?= 2
REGISTRY ?= ghcr.io/$(PROJECT_ORG)/$(PROJECT_NAME)
LDFLAGS := -s -w -X 'main.Version=$(VERSION)' -X 'main.CommitID=$(COMMIT_ID)' -X 'main.BuildEpoch=$(BUILD_EPOCH)' -X 'main.OfficialSite=$(OFFICIAL_SITE)'
CLIENT_LDFLAGS := -s -w -X 'main.Version=$(VERSION)' -X 'main.CommitID=$(COMMIT_ID)' -X 'main.BuildEpoch=$(BUILD_EPOCH)'
CONTAINER := docker run --rm --name $(PROJECT_NAME)-$$(tr -dc 'a-z0-9' </dev/urandom | head -c8) --memory=$(DOCKER_MEM) --cpus=$(DOCKER_CPUS) -v $(PWD):/app -v $(GO_CACHE):/usr/local/share/go/pkg/mod -v $(GO_BUILD):/usr/local/share/go/cache -w /app -e CGO_ENABLED=0 -e GOFLAGS=-buildvcs=false casjaysdev/go:latest

.PHONY: dev local build test release docker clean help

dev:
	@mkdir -p "$(GO_CACHE)" "$(GO_BUILD)" "$${TMPDIR:-/tmp}/$(PROJECT_ORG)"
	@$(CONTAINER) go mod tidy
	@tmpdir=$$(mktemp -d "$${TMPDIR:-/tmp}/$(PROJECT_ORG)/$(INTERNAL_NAME)-XXXXXX") && \
	docker run --rm --name $(PROJECT_NAME)-$$(tr -dc 'a-z0-9' </dev/urandom | head -c8) --memory=$(DOCKER_MEM) --cpus=$(DOCKER_CPUS) -v "$(PWD):/app" -v "$$tmpdir:/out" -v "$(GO_CACHE):/usr/local/share/go/pkg/mod" -v "$(GO_BUILD):/usr/local/share/go/cache" -w /app -e CGO_ENABLED=0 -e GOFLAGS=-buildvcs=false casjaysdev/go:latest sh -c 'go build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o /out/$(PROJECT_NAME) ./src && go build -buildvcs=false -trimpath -ldflags "$(CLIENT_LDFLAGS)" -o /out/$(PROJECT_NAME)-cli ./src/client' && echo "Development binaries: $$tmpdir/$(PROJECT_NAME) and $$tmpdir/$(PROJECT_NAME)-cli"

local:
	@mkdir -p $(BINDIR) $(GO_CACHE) $(GO_BUILD)
	@$(CONTAINER) go mod tidy
	@$(CONTAINER) sh -c 'go build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(PROJECT_NAME) ./src && go build -buildvcs=false -trimpath -ldflags "$(CLIENT_LDFLAGS)" -o $(BINDIR)/$(PROJECT_NAME)-cli ./src/client'

build:
	@mkdir -p $(BINDIR) $(GO_CACHE) $(GO_BUILD)
	@$(CONTAINER) go mod tidy
	@for platform in $$(echo "$(PLATFORMS)" | tr ',' ' '); do \
		os=$${platform%/*}; arch=$${platform#*/}; ext=; [ "$$os" = windows ] && ext=.exe; \
		$(CONTAINER) sh -c "GOOS=$$os GOARCH=$$arch go build -buildvcs=false -trimpath -ldflags \"$(LDFLAGS)\" -o $(BINDIR)/$(PROJECT_NAME)-$$os-$$arch$$ext ./src && GOOS=$$os GOARCH=$$arch go build -buildvcs=false -trimpath -ldflags \"$(CLIENT_LDFLAGS)\" -o $(BINDIR)/$(PROJECT_NAME)-cli-$$os-$$arch$$ext ./src/client" || exit 1; \
	done

test:
	@mkdir -p $(GO_CACHE) $(GO_BUILD) "$${TMPDIR:-/tmp}/$(PROJECT_ORG)"
	@$(CONTAINER) sh -c '		mkdir -p "$${TMPDIR:-/tmp}/$(PROJECT_ORG)" && 		covdir=$$(mktemp -d "$${TMPDIR:-/tmp}/$(PROJECT_ORG)/$(INTERNAL_NAME)-XXXXXX") && 		trap "rm -rf $$covdir" EXIT && 		go vet ./... && govulncheck ./... && 		go test -timeout 5m -covermode=atomic -coverprofile=$$covdir/coverage.out ./... && 		go tool cover -func=$$covdir/coverage.out && 		coverage=$$(go tool cover -func=$$covdir/coverage.out | awk "/^total:/ {gsub(/%/, \"\", \$$3); print \$$3}") && 		awk -v coverage="$$coverage" "BEGIN { if (coverage+0 < 60) exit 1 }" 	'

release: build
	@mkdir -p $(RELDIR)
	@cp $(BINDIR)/* $(RELDIR)/
	@tar --exclude=.git --exclude=binaries --exclude=releases -czf $(RELDIR)/$(PROJECT_NAME)-$(VERSION)-source.tar.gz .

docker:
	@docker buildx build --platform linux/amd64,linux/arm64 --file docker/Dockerfile --build-arg VERSION=$(VERSION) --build-arg BUILD_EPOCH=$(BUILD_EPOCH) --build-arg COMMIT_ID=$(COMMIT_ID) --build-arg OFFICIAL_SITE=$(OFFICIAL_SITE) --tag $(REGISTRY):$(VERSION) --tag $(REGISTRY):latest --push .

clean:
	@rm -rf $(BINDIR) $(RELDIR)

help:
	@printf '%s\n' 'dev local build test release docker clean'
