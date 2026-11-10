FROM casjaysdev/go:latest AS builder
ARG TARGETARCH
ARG VERSION=devel
ARG BUILD_EPOCH=0
ARG COMMIT_ID=unknown
ARG OFFICIAL_SITE=
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -buildvcs=false -trimpath -ldflags "-s -w -X 'main.Version=${VERSION}' -X 'main.BuildEpoch=${BUILD_EPOCH}' -X 'main.CommitID=${COMMIT_ID}' -X 'main.OfficialSite=${OFFICIAL_SITE}'" -o /build/anime ./src && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -buildvcs=false -trimpath -ldflags "-s -w -X 'main.Version=${VERSION}' -X 'main.CommitID=${COMMIT_ID}' -X 'main.BuildEpoch=${BUILD_EPOCH}'" -o /build/anime-cli ./src/client

FROM alpine:latest
RUN apk add --no-cache git curl bash tini tor
COPY --from=builder /build/anime /usr/local/bin/anime
COPY --from=builder /build/anime-cli /usr/local/bin/anime-cli
COPY docker/rootfs/ /
ENV PORT=80 ADDRESS=:: CONFIG_DIR=/config/anime DATA_DIR=/data/anime LOGS_DIR=/data/log/anime
EXPOSE 80
STOPSIGNAL SIGRTMIN+3
HEALTHCHECK --start-period=10m --interval=5m --timeout=15s --retries=3 CMD ["/usr/local/bin/anime", "--status"]
ENTRYPOINT ["tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh"]
