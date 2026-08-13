# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.4
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /src

# 各 Go module 使用本地 replace，构建上下文必须包含 common、proto、services 和 API。
COPY common ./common
COPY proto ./proto
COPY services ./services
COPY appforge-api ./appforge-api

ARG MODULE_DIR
WORKDIR /src/${MODULE_DIR}

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/appforge .

FROM alpine:3.21

RUN addgroup -S appforge && adduser -S -G appforge appforge
COPY --from=builder /out/appforge /usr/local/bin/appforge

USER appforge
ENTRYPOINT ["/usr/local/bin/appforge"]
