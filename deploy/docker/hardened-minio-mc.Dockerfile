# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.6
FROM golang:${GO_VERSION}-bookworm AS builder

ARG MC_COMMIT=7394ce0dd2a80935aded936b09fa12cbb3cb8096
WORKDIR /src/mc
RUN git init \
    && git remote add origin https://github.com/minio/mc.git \
    && git fetch --depth 1 origin "$MC_COMMIT" \
    && git checkout --detach FETCH_HEAD \
    && go mod edit -go=1.26.6 -toolchain=go1.26.6 \
    && go get \
      github.com/prometheus/prometheus@v0.311.3 \
      golang.org/x/crypto@v0.54.0 \
      golang.org/x/net@v0.57.0 \
      golang.org/x/text@v0.40.0 \
      google.golang.org/grpc@v1.82.1 \
    && go mod tidy \
    && LDFLAGS="$(MC_RELEASE=APPFORGE go run buildscripts/gen-ldflags.go)" \
    && CGO_ENABLED=0 go build -trimpath -tags kqueue -ldflags="$LDFLAGS" -o /out/mc .

FROM alpine:3.23
ARG MC_COMMIT=7394ce0dd2a80935aded936b09fa12cbb3cb8096
LABEL io.appforge.upstream.repository="https://github.com/minio/mc" \
      io.appforge.upstream.commit="$MC_COMMIT"
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 65532 appforge \
    && adduser -S -D -H -u 65532 -G appforge appforge
COPY --from=builder /out/mc /usr/local/bin/mc
USER appforge
ENTRYPOINT ["/usr/local/bin/mc"]
