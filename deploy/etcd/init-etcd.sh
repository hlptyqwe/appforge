#!/bin/sh
set -eu

export ETCDCTL_API=3

until etcdctl --endpoints=http://etcd:2379 endpoint health >/dev/null 2>&1; do
  sleep 1
done

etcdctl --endpoints=http://etcd:2379 put /appforge/common/config < /config/common.yaml
etcdctl --endpoints=http://etcd:2379 put /appforge/system-rpc/config < /config/system.yaml
etcdctl --endpoints=http://etcd:2379 put /appforge/core-rpc/config < /config/core.yaml
etcdctl --endpoints=http://etcd:2379 put /appforge/builder-rpc/config < /config/builder.yaml
etcdctl --endpoints=http://etcd:2379 put /appforge/admin-api/config < /config/admin-api.yaml

echo "AppForge development configuration has been written to etcd."

