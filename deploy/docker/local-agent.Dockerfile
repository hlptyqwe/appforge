# syntax=docker/dockerfile:1.7
ARG GO_VERSION=1.26.4
FROM golang:${GO_VERSION}-bookworm AS builder
WORKDIR /src
COPY common ./common
COPY proto ./proto
COPY services ./services
COPY local-agent ./local-agent
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd /src/local-agent \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/appforge-local-agent . \
    && cd /src/services/builder \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/appforge-local-build ./cmd/local-build

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends aapt apksigner apktool zipalign imagemagick default-jre-headless ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 65532 appforge \
    && useradd --system --uid 65532 --gid 65532 --create-home appforge \
    && mkdir -p /tmp/appforge-local-agent /etc/appforge/local-secrets /var/lib/appforge-agent \
    && chown appforge:appforge /tmp/appforge-local-agent /etc/appforge/local-secrets /var/lib/appforge-agent \
    && chmod 0700 /tmp/appforge-local-agent /etc/appforge/local-secrets /var/lib/appforge-agent
COPY --from=builder /out/appforge-local-agent /usr/local/bin/appforge-local-agent
COPY --from=builder /out/appforge-local-build /usr/local/bin/appforge-local-build
USER appforge
ENTRYPOINT ["/usr/local/bin/appforge-local-agent"]
