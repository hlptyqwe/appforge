# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.6
FROM golang:${GO_VERSION}-bookworm AS builder

ARG MINIO_COMMIT=9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a
WORKDIR /src/minio
RUN git init \
    && git remote add origin https://github.com/minio/minio.git \
    && git fetch --depth 1 origin "$MINIO_COMMIT" \
    && git checkout --detach FETCH_HEAD \
    && go mod edit -go=1.26.6 -toolchain=go1.26.6 \
    && go get \
      github.com/apache/thrift@v0.23.0 \
      github.com/buger/jsonparser@v1.1.2 \
      github.com/go-jose/go-jose/v4@v4.1.4 \
      github.com/prometheus/prometheus@v0.311.3 \
      go.opentelemetry.io/otel/sdk@v1.43.0 \
      golang.org/x/crypto@v0.54.0 \
      golang.org/x/net@v0.57.0 \
      golang.org/x/text@v0.40.0 \
      google.golang.org/grpc@v1.82.1 \
    && LDFLAGS="$(MINIO_RELEASE=APPFORGE go run buildscripts/gen-ldflags.go)" \
    && CGO_ENABLED=0 go build -mod=mod -trimpath -tags kqueue -ldflags="$LDFLAGS" -o /out/minio .

FROM alpine:3.23
ARG MINIO_COMMIT=9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a
LABEL io.appforge.upstream.repository="https://github.com/minio/minio" \
      io.appforge.upstream.commit="$MINIO_COMMIT"
RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /data
COPY --from=builder /out/minio /usr/local/bin/minio
EXPOSE 9000 9001
ENTRYPOINT ["/usr/local/bin/minio"]
