#!/bin/sh
set -eu

export ETCDCTL_API=3
ETCD_ENDPOINTS=${ETCD_ENDPOINTS:-http://etcd:2379}

until etcdctl --endpoints="$ETCD_ENDPOINTS" endpoint health >/dev/null 2>&1; do
  sleep 1
done

etcdctl --endpoints="$ETCD_ENDPOINTS" put /appforge/common/config < /config/common.yaml
etcdctl --endpoints="$ETCD_ENDPOINTS" put /appforge/system-rpc/config < /config/system.yaml
etcdctl --endpoints="$ETCD_ENDPOINTS" put /appforge/core-rpc/config < /config/core.yaml
etcdctl --endpoints="$ETCD_ENDPOINTS" put /appforge/builder-rpc/config < /config/builder.yaml
etcdctl --endpoints="$ETCD_ENDPOINTS" put /appforge/admin-api/config < /config/admin-api.yaml

echo "AppForge runtime configuration has been written to etcd."
