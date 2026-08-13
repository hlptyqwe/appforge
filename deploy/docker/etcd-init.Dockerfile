FROM quay.io/coreos/etcd:v3.6.12 AS etcd

FROM alpine:3.21
COPY --from=etcd /usr/local/bin/etcdctl /usr/local/bin/etcdctl

ENTRYPOINT ["/bin/sh", "/init/init-etcd.sh"]
