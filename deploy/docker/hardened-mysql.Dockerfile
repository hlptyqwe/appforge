# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.6
FROM golang:${GO_VERSION}-bookworm AS gosu-builder

ARG GOSU_COMMIT=6456aaa0f3c854d199d0f037f068eb97515b7513
WORKDIR /src/gosu
RUN git init \
    && git remote add origin https://github.com/tianon/gosu.git \
    && git fetch --depth 1 origin "$GOSU_COMMIT" \
    && git checkout --detach FETCH_HEAD \
    && go mod edit -go=1.26.6 \
    && go get golang.org/x/sys@v0.47.0 \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gosu .

FROM mysql:8.4

ARG GOSU_COMMIT=6456aaa0f3c854d199d0f037f068eb97515b7513
LABEL io.appforge.upstream.repository="https://github.com/docker-library/mysql" \
      io.appforge.gosu.commit="$GOSU_COMMIT"
USER root
RUN microdnf remove -y mysql-shell \
    && microdnf clean all \
    && rm -rf /var/cache/dnf
COPY --from=gosu-builder /out/gosu /usr/local/bin/gosu
RUN gosu nobody true
