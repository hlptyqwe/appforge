# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.6
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /src
COPY common ./common
COPY proto ./proto
COPY services ./services

WORKDIR /src/services/builder
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/appforge-builder-worker ./cmd/worker

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends aapt apksigner apktool zipalign imagemagick default-jre-headless ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 65532 appforge \
    && useradd --system --uid 65532 --gid 65532 --create-home appforge \
    && mkdir -p /tmp/appforge-builder \
    && chown appforge:appforge /tmp/appforge-builder

COPY --from=builder /out/appforge-builder-worker /usr/local/bin/appforge-builder-worker

USER appforge
ENTRYPOINT ["/usr/local/bin/appforge-builder-worker"]
