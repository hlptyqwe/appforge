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
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/appforge-builder-rpc .

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends default-jre-headless ca-certificates netcat-openbsd \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 65532 appforge \
    && useradd --system --uid 65532 --gid 65532 --create-home appforge

COPY --from=builder /out/appforge-builder-rpc /usr/local/bin/appforge-builder-rpc

USER appforge
ENTRYPOINT ["/usr/local/bin/appforge-builder-rpc"]
