FROM quay.io/coreos/etcd:v3.6.12 AS etcd

FROM alpine:3.21
COPY --from=etcd /usr/local/bin/etcdctl /usr/local/bin/etcdctl
COPY deploy/etcd/init-etcd.sh /init/init-etcd.sh

ENTRYPOINT ["/bin/sh", "/init/init-etcd.sh"]
