# ============================================
# Build stage
# ============================================
FROM golang:alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN apk add --no-cache git make ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY src/ ./src/

# Build static binary with all assets embedded
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE} -w -s" \
    -a -installsuffix cgo \
    -o anime \
    ./src

# ============================================
# Runtime stage - Alpine with minimal tools
# ============================================
FROM alpine:latest

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Install runtime dependencies (curl, bash)
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    bash \
    && rm -rf /var/cache/apk/*

# Copy binary to /usr/local/bin
COPY --from=builder /build/anime /usr/local/bin/anime
RUN chmod +x /usr/local/bin/anime

# Environment variables
ENV PORT=80 \
    CONFIG_DIR=/config \
    DATA_DIR=/data \
    LOGS_DIR=/logs \
    ADDRESS=0.0.0.0 \
    DB_PATH=/data/db/anime.db

# Create directories
RUN mkdir -p /config /data /data/db /logs && \
    chown -R 65534:65534 /config /data /logs

# Metadata labels (OCI standard)
LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.authors="apimgr" \
      org.opencontainers.image.url="https://github.com/apimgr/anime" \
      org.opencontainers.image.source="https://github.com/apimgr/anime" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.vendor="apimgr" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.title="anime" \
      org.opencontainers.image.description="Anime Quotes API - Single static binary" \
      org.opencontainers.image.documentation="https://github.com/apimgr/anime/blob/main/README.md" \
      org.opencontainers.image.base.name="alpine:latest"

# Expose default port
EXPOSE 80

# Create mount points for volumes
VOLUME ["/config", "/data", "/logs"]

# Run as non-root user (nobody)
USER 65534:65534

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/anime", "--status"]

# Run
ENTRYPOINT ["/usr/local/bin/anime"]
CMD ["--port", "80"]
