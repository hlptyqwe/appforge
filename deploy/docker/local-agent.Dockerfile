# syntax=docker/dockerfile:1.7
ARG GO_VERSION=1.26.4
FROM golang:${GO_VERSION}-bookworm AS builder
WORKDIR /src/local-agent
COPY local-agent/go.mod ./
COPY local-agent/main.go ./
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/appforge-local-agent .

FROM alpine:3.22
RUN addgroup -S appforge && adduser -S -G appforge appforge
COPY --from=builder /out/appforge-local-agent /usr/local/bin/appforge-local-agent
USER appforge
ENTRYPOINT ["/usr/local/bin/appforge-local-agent"]
