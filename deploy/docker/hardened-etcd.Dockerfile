# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.6
FROM golang:${GO_VERSION}-bookworm AS builder

ARG ETCD_COMMIT=5e7fd0de9a57db03ecc11794dc40403a734c07bb
WORKDIR /src/etcd
RUN git init \
    && git remote add origin https://github.com/etcd-io/etcd.git \
    && git fetch --depth 1 origin "$ETCD_COMMIT" \
    && git checkout --detach FETCH_HEAD \
    && go work edit -go=1.26.6 -toolchain=go1.26.6 \
    && go mod edit -go=1.26.6 -toolchain=go1.26.6 \
    && go get golang.org/x/net@v0.57.0 golang.org/x/text@v0.40.0 \
    && GOTOOLCHAIN=go1.26.6 go work sync \
    && GO_VERSION=1.26.6 GOTOOLCHAIN=go1.26.6 GOOS=linux CGO_ENABLED=0 ./scripts/build.sh

FROM alpine:3.23
ARG ETCD_COMMIT=5e7fd0de9a57db03ecc11794dc40403a734c07bb
LABEL io.appforge.upstream.repository="https://github.com/etcd-io/etcd" \
      io.appforge.upstream.commit="$ETCD_COMMIT"
RUN apk add --no-cache ca-certificates
COPY --from=builder /src/etcd/bin/etcd /usr/local/bin/etcd
COPY --from=builder /src/etcd/bin/etcdctl /usr/local/bin/etcdctl
COPY --from=builder /src/etcd/bin/etcdutl /usr/local/bin/etcdutl
ENTRYPOINT ["/usr/local/bin/etcd"]
