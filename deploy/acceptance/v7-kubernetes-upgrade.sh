#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
kind_bin=${APPFORGE_KIND_BIN:-kind}
helm_bin=${APPFORGE_HELM_BIN:-helm}
kubectl_bin=${APPFORGE_KUBECTL_BIN:-kubectl}
cluster=${APPFORGE_V7_KIND_CLUSTER:-appforge-v7-${RANDOM}-$$}
context="kind-$cluster"
namespace=${APPFORGE_V7_KUBERNETES_NAMESPACE:-appforge-v7-acceptance}
license_pv="${namespace}-license-state"
registry=${APPFORGE_V7_KUBERNETES_REGISTRY:-appforge-private-acceptance}
old_version=${APPFORGE_V7_KUBERNETES_OLD_VERSION:-1.2.0}
new_version=${APPFORGE_V7_KUBERNETES_NEW_VERSION:-1.2.1}
report_file=${APPFORGE_V7_KUBERNETES_REPORT_FILE:-}
node_image=${APPFORGE_V7_KIND_NODE_IMAGE:-kindest/node:v1.32.2}
go_cache=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache}
temporary=$(mktemp -d /tmp/appforge-v7-kubernetes.XXXXXX)
chart_dir="$temporary/appforge-chart"
created_cluster=false
probe_started=false

components=(system core builder builder-worker api admin-ui agent-ui etcd-init migrate)
dependencies=(mysql:8.4 redis:7.4-alpine quay.io/coreos/etcd:v3.6.12 minio/minio:RELEASE.2025-04-22T22-12-26Z alpine:3.22)
created_old_components=()

cleanup() {
  set +e
  if [[ $probe_started == true ]]; then
    "$kubectl_bin" --context "$context" -n "$namespace" delete pod appforge-availability-probe --ignore-not-found --wait=false >/dev/null 2>&1 || true
  fi
  "$kubectl_bin" --context "$context" delete namespace "$namespace" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  "$kubectl_bin" --context "$context" delete persistentvolume "$license_pv" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  for component in "${components[@]}"; do
    docker image rm "$registry/$component:$new_version" >/dev/null 2>&1 || true
  done
  for component in "${created_old_components[@]}"; do
    docker image rm "$registry/$component:$old_version" >/dev/null 2>&1 || true
  done
  if [[ $created_cluster == true ]]; then
    "$kind_bin" delete cluster --name "$cluster" >/dev/null 2>&1 || true
  fi
  rm -rf "$temporary"
}
trap cleanup EXIT

for command_path in "$kind_bin" "$helm_bin" "$kubectl_bin" docker openssl htpasswd go; do
  command -v "$command_path" >/dev/null || { echo "验收失败: 缺少命令 $command_path" >&2; exit 1; }
done
cp -R "$repo_root/deploy/helm/appforge" "$chart_dir"
find "$chart_dir/templates" -type f -name '*.yaml' -exec sed -i.bak \
  's/emptyDir: {sizeLimit:/emptyDir: {medium: Memory, sizeLimit:/g' {} +
find "$chart_dir/templates" -type f -name '*.bak' -delete
if ! docker image inspect "$registry/migrate:$old_version" >/dev/null 2>&1; then
  docker build --pull=false --build-arg APPFORGE_SCHEMA_TARGET=20260815_113_v7_air_gapped \
    -f "$repo_root/deploy/docker/migrate.Dockerfile" -t "$registry/migrate:$old_version" "$repo_root" >/dev/null
  created_old_components+=("migrate")
fi
for component in "${components[@]}"; do
  docker image inspect "$registry/$component:$old_version" >/dev/null 2>&1 || {
    echo "验收失败: 缺少 Kubernetes 验收镜像 $registry/$component:$old_version" >&2
    exit 1
  }
  docker image tag "$registry/$component:$old_version" "$registry/$component:$new_version"
done
for dependency in "${dependencies[@]}"; do
  docker image inspect "$dependency" >/dev/null 2>&1 || { echo "验收失败: 缺少依赖镜像 $dependency" >&2; exit 1; }
done

if ! "$kind_bin" get clusters | grep -Fxq "$cluster"; then
  "$kind_bin" create cluster --name "$cluster" --image "$node_image" --wait 120s
  created_cluster=true
fi
"$kubectl_bin" --context "$context" wait --for=condition=Ready node --all --timeout=120s >/dev/null
for version in "$old_version" "$new_version"; do
  images=()
  for component in "${components[@]}"; do images+=("$registry/$component:$version"); done
  "$kind_bin" load docker-image --name "$cluster" "${images[@]}" >/dev/null
done
"$kind_bin" load docker-image --name "$cluster" "${dependencies[@]}" >/dev/null

"$kubectl_bin" --context "$context" create namespace "$namespace" >/dev/null
"$kubectl_bin" --context "$context" -n "$namespace" apply -f - <<'YAML' >/dev/null
apiVersion: v1
kind: Service
metadata: {name: mysql}
spec: {selector: {app: mysql}, ports: [{port: 3306, targetPort: 3306}]}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: mysql}
spec:
  replicas: 1
  selector: {matchLabels: {app: mysql}}
  template:
    metadata: {labels: {app: mysql}}
    spec:
      containers:
        - name: mysql
          image: mysql:8.4
          imagePullPolicy: Never
          args: ["--character-set-server=utf8mb4", "--collation-server=utf8mb4_0900_ai_ci"]
          env:
            - {name: MYSQL_DATABASE, value: appforge}
            - {name: MYSQL_USER, value: appforge}
            - {name: MYSQL_PASSWORD, value: kubernetes_acceptance_mysql}
            - {name: MYSQL_ROOT_PASSWORD, value: kubernetes_acceptance_root}
          ports: [{containerPort: 3306}]
          readinessProbe: {exec: {command: ["sh", "-c", "MYSQL_PWD=$MYSQL_ROOT_PASSWORD mysqladmin ping -h 127.0.0.1 -uroot --silent"]}, periodSeconds: 2, failureThreshold: 60}
          volumeMounts: [{name: data, mountPath: /var/lib/mysql}]
      volumes:
        - name: data
          emptyDir: {medium: Memory, sizeLimit: 768Mi}
---
apiVersion: v1
kind: Service
metadata: {name: redis}
spec: {selector: {app: redis}, ports: [{port: 6379, targetPort: 6379}]}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: redis}
spec:
  replicas: 1
  selector: {matchLabels: {app: redis}}
  template:
    metadata: {labels: {app: redis}}
    spec:
      containers:
        - name: redis
          image: redis:7.4-alpine
          imagePullPolicy: Never
          command: ["sh", "-c", "exec redis-server --requirepass kubernetes_acceptance_redis"]
          ports: [{containerPort: 6379}]
          readinessProbe: {exec: {command: ["sh", "-c", "REDISCLI_AUTH=kubernetes_acceptance_redis redis-cli ping | grep PONG"]}, periodSeconds: 2}
---
apiVersion: v1
kind: Service
metadata: {name: etcd}
spec: {selector: {app: etcd}, ports: [{port: 2379, targetPort: 2379}]}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: etcd}
spec:
  replicas: 1
  selector: {matchLabels: {app: etcd}}
  template:
    metadata: {labels: {app: etcd}}
    spec:
      containers:
        - name: etcd
          image: quay.io/coreos/etcd:v3.6.12
          imagePullPolicy: Never
          command: ["/usr/local/bin/etcd"]
          args: ["--name=appforge-etcd", "--data-dir=/etcd-data", "--listen-client-urls=http://0.0.0.0:2379", "--advertise-client-urls=http://etcd:2379"]
          ports: [{containerPort: 2379}]
          readinessProbe: {exec: {command: ["/usr/local/bin/etcdctl", "--endpoints=http://127.0.0.1:2379", "endpoint", "health"]}, periodSeconds: 2}
          volumeMounts: [{name: data, mountPath: /etcd-data}]
      volumes: [{name: data, emptyDir: {}}]
---
apiVersion: v1
kind: Service
metadata: {name: minio}
spec: {selector: {app: minio}, ports: [{port: 9000, targetPort: 9000}]}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: minio}
spec:
  replicas: 1
  selector: {matchLabels: {app: minio}}
  template:
    metadata: {labels: {app: minio}}
    spec:
      containers:
        - name: minio
          image: minio/minio:RELEASE.2025-04-22T22-12-26Z
          imagePullPolicy: Never
          args: ["server", "/data"]
          env:
            - {name: MINIO_ROOT_USER, value: kubernetes_acceptance_minio}
            - {name: MINIO_ROOT_PASSWORD, value: kubernetes_acceptance_minio_secret}
          ports: [{containerPort: 9000}]
          readinessProbe: {httpGet: {path: /minio/health/ready, port: 9000}, periodSeconds: 2}
          volumeMounts: [{name: data, mountPath: /data}]
      volumes: [{name: data, emptyDir: {}}]
YAML
printf '%s\n' \
  'apiVersion: v1' 'kind: PersistentVolume' "metadata: {name: $license_pv}" \
  'spec:' '  capacity: {storage: 64Mi}' '  accessModes: [ReadWriteMany]' '  persistentVolumeReclaimPolicy: Delete' \
  '  storageClassName: ""' "  hostPath: {path: /tmp/$license_pv, type: DirectoryOrCreate}" \
  '---' 'apiVersion: v1' 'kind: PersistentVolumeClaim' \
  "metadata: {name: appforge-license-state, namespace: $namespace}" \
  'spec:' '  accessModes: [ReadWriteMany]' '  storageClassName: ""' "  volumeName: $license_pv" \
  '  resources: {requests: {storage: 64Mi}}' |
  "$kubectl_bin" --context "$context" apply -f - >/dev/null
for dependency in mysql redis etcd minio; do
  "$kubectl_bin" --context "$context" -n "$namespace" rollout status "deployment/$dependency" --timeout=180s >/dev/null
done
"$kubectl_bin" --context "$context" -n "$namespace" apply -f - <<'YAML' >/dev/null
apiVersion: batch/v1
kind: Job
metadata: {name: appforge-license-state-init}
spec:
  backoffLimit: 1
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: permissions
          image: alpine:3.22
          imagePullPolicy: Never
          command: ["sh", "-c", "chown 65532:65532 /state && chmod 0700 /state"]
          securityContext: {runAsUser: 0, runAsGroup: 0}
          volumeMounts: [{name: state, mountPath: /state}]
      volumes:
        - name: state
          persistentVolumeClaim: {claimName: appforge-license-state}
YAML
"$kubectl_bin" --context "$context" -n "$namespace" wait --for=condition=Complete \
  job/appforge-license-state-init --timeout=60s >/dev/null
"$kubectl_bin" --context "$context" -n "$namespace" delete job appforge-license-state-init --wait=true >/dev/null

admin_password='Acceptance-Kubernetes-Password-2026!'
bcrypt_base64=$(htpasswd -bnBC 10 '' "$admin_password" | tr -d ':\n' | base64 | tr -d '\n')
master_key=$(openssl rand -base64 32 | tr -d '\n')
mkdir -p "$temporary/production/runtime" "$temporary/production/secrets"
cp "$repo_root/deploy/production/render-config.sh" "$temporary/production/render-config.sh"
cp "$repo_root/deploy/etcd/admin-api.yaml" "$temporary/admin-api.yaml"
printf '%s\n' \
  'APPFORGE_DEPLOYMENT_MODE=private' \
  'APPFORGE_PUBLIC_ORIGIN=https://appforge.acceptance.local' \
  'APPFORGE_MYSQL_DATABASE=appforge' \
  'APPFORGE_MYSQL_USER=appforge' \
  'APPFORGE_MYSQL_PASSWORD=kubernetes_acceptance_mysql' \
  'APPFORGE_REDIS_PASSWORD=kubernetes_acceptance_redis' \
  'APPFORGE_MINIO_ACCESS_KEY=kubernetes_acceptance_minio' \
  'APPFORGE_MINIO_SECRET_KEY=kubernetes_acceptance_minio_secret' \
  'APPFORGE_INTERNAL_RPC_TOKEN=kubernetes_acceptance_internal_rpc' \
  'APPFORGE_JWT_ACCESS_SECRET=kubernetes_acceptance_jwt_secret_32_bytes' \
  "APPFORGE_SECRET_MASTER_KEY_BASE64=$master_key" \
  'APPFORGE_DEPLOYMENT_ID=kubernetes-upgrade-acceptance' \
  'APPFORGE_LOCAL_AGENT_CA_CERT_FILE=unused' \
  'APPFORGE_LOCAL_AGENT_CA_KEY_FILE=unused' \
  'APPFORGE_LICENSE_FILE=unused' \
  'APPFORGE_LICENSE_PUBLIC_KEY_FILE=unused' \
  'APPFORGE_SIEM_ENDPOINT=https://127.0.0.1:9/audit' \
  'APPFORGE_SIEM_TOKEN_FILE=unused' \
  'APPFORGE_SIEM_CA_FILE=unused' >"$temporary/production/.env"
chmod 0600 "$temporary/production/.env"
mkdir -p "$temporary/etcd" "$temporary/deploy"
cp "$temporary/admin-api.yaml" "$temporary/etcd/admin-api.yaml"
cp "$temporary/production/render-config.sh" "$temporary/deploy/render-config.sh"
(
  cd "$temporary/deploy"
  ./render-config.sh "$temporary/production/.env" >/dev/null
)
runtime_dir="$temporary/deploy/runtime"
sed -i.bak -e 's/system-rpc:8080/appforge-system:8080/g' -e 's/core-rpc:8081/appforge-core:8081/g' \
  -e 's/builder-rpc:8082/appforge-builder:8082/g' "$runtime_dir/admin-api.yaml" "$runtime_dir/builder.yaml"
rm -f "$runtime_dir/"*.bak

openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj '/CN=appforge-api' \
  -addext 'subjectAltName=DNS:appforge-api,DNS:localhost' -keyout "$temporary/tls.key" -out "$temporary/tls.crt" >/dev/null 2>&1
openssl ecparam -name prime256v1 -genkey -noout -out "$temporary/agent-ca.key"
openssl req -x509 -new -key "$temporary/agent-ca.key" -days 1 -subj '/CN=AppForge Kubernetes Agent CA' \
  -addext 'basicConstraints=critical,CA:TRUE' -addext 'keyUsage=critical,keyCertSign,cRLSign' \
  -out "$temporary/agent-ca.crt" >/dev/null 2>&1
(
  cd "$repo_root/appforge-api"
  GOCACHE="$go_cache" go run ./cmd/licensectl generate-key --private "$temporary/vendor-private.pem" \
    --public "$temporary/vendor-public.pem" >/dev/null
  GOCACHE="$go_cache" go run ./cmd/licensectl issue --private "$temporary/vendor-private.pem" \
    --output "$temporary/license.json" --license-id kubernetes-upgrade-acceptance --customer acceptance \
    --deployment-id kubernetes-upgrade-acceptance --modes private --valid-for 24h >/dev/null
)

"$kubectl_bin" --context "$context" -n "$namespace" create secret generic appforge-etcd-client \
  --from-literal=ETCD_USERNAME= --from-literal=ETCD_PASSWORD= >/dev/null
"$kubectl_bin" --context "$context" -n "$namespace" create secret generic appforge-runtime-config \
  --from-file=common.yaml="$runtime_dir/common.yaml" --from-file=system.yaml="$runtime_dir/system.yaml" \
  --from-file=core.yaml="$runtime_dir/core.yaml" --from-file=builder.yaml="$runtime_dir/builder.yaml" \
  --from-file=admin-api.yaml="$runtime_dir/admin-api.yaml" >/dev/null
"$kubectl_bin" --context "$context" -n "$namespace" create secret generic appforge-migration \
  --from-literal=MYSQL_HOST=mysql --from-literal=MYSQL_DATABASE=appforge --from-literal=MYSQL_USER=appforge \
  --from-literal=MYSQL_PASSWORD=kubernetes_acceptance_mysql --from-literal=APPFORGE_BOOTSTRAP_ADMIN_USERNAME=owner \
  --from-literal=APPFORGE_BOOTSTRAP_PASSWORD_BCRYPT_BASE64="$bcrypt_base64" \
  --from-literal=APPFORGE_MINIO_ACCESS_KEY=kubernetes_acceptance_minio \
  --from-literal=APPFORGE_MINIO_SECRET_KEY=kubernetes_acceptance_minio_secret >/dev/null
"$kubectl_bin" --context "$context" -n "$namespace" create secret generic appforge-local-agent-ca \
  --from-file=ca.crt="$temporary/agent-ca.crt" --from-file=ca.key="$temporary/agent-ca.key" >/dev/null
"$kubectl_bin" --context "$context" -n "$namespace" create secret tls appforge-tls \
  --cert="$temporary/tls.crt" --key="$temporary/tls.key" >/dev/null
"$kubectl_bin" --context "$context" -n "$namespace" create secret generic appforge-offline-license \
  --from-file=license.json="$temporary/license.json" --from-file=vendor-public.pem="$temporary/vendor-public.pem" >/dev/null
"$kubectl_bin" --context "$context" -n "$namespace" create secret generic appforge-siem \
  --from-literal=token=kubernetes-acceptance-siem --from-file=ca.crt="$temporary/tls.crt" >/dev/null

printf '%s\n' \
  'global:' "  deploymentMode: private" "  imageRegistry: $registry" '  publicOrigin: https://appforge.acceptance.local' '  offline: true' \
  'image:' "  tag: $old_version" '  pullPolicy: Never' \
  'replicas:' '  api: 2' '  system: 2' '  core: 2' '  builder: 1' '  builderWorker: 1' '  worker: 1' '  sourceWorker: 1' '  ui: 2' \
  'external:' '  etcdEndpoints: http://etcd:2379' '  etcdAuthSecret: appforge-etcd-client' \
  '  runtimeConfigSecret: appforge-runtime-config' '  migrationSecret: appforge-migration' \
  'offlineLicense:' '  enabled: true' '  deploymentId: kubernetes-upgrade-acceptance' '  existingStateClaim: appforge-license-state' \
  'ingress:' '  enabled: false' '  className: nginx' '  host: appforge.acceptance.local' '  adminHost: admin.appforge.acceptance.local' '  tlsSecretName: appforge-tls' \
  'localAgentGateway:' '  enabled: true' '  serviceType: ClusterIP' '  port: 9443' \
  'networkPolicy:' '  enabled: false' \
  'observability:' '  prometheus: false' '  otlpEndpoint: ""' '  siemWebhook: https://127.0.0.1:9/audit' \
  'resources:' \
  '  api: {requests: {cpu: 10m, memory: 32Mi}, limits: {cpu: 500m, memory: 256Mi}}' \
  '  rpc: {requests: {cpu: 10m, memory: 32Mi}, limits: {cpu: 500m, memory: 256Mi}}' \
  '  worker: {requests: {cpu: 10m, memory: 64Mi}, limits: {cpu: "1", memory: 512Mi}}' \
  '  ui: {requests: {cpu: 5m, memory: 16Mi}, limits: {cpu: 250m, memory: 128Mi}}' \
  >"$temporary/values.yaml"

"$helm_bin" --kube-context "$context" upgrade --install appforge "$chart_dir" \
  --namespace "$namespace" -f "$temporary/values.yaml" --atomic --timeout 10m >/dev/null
deployments=(deployment/appforge-api deployment/appforge-system deployment/appforge-core deployment/appforge-builder \
  deployment/appforge-builder-worker deployment/appforge-webhook-worker deployment/appforge-billing-worker \
  deployment/appforge-enterprise-worker deployment/appforge-source-trigger-worker deployment/appforge-admin-ui \
  deployment/appforge-agent-ui)
for deployment in "${deployments[@]}"; do
  "$kubectl_bin" --context "$context" -n "$namespace" rollout status "$deployment" --timeout=300s >/dev/null
done

"$kubectl_bin" --context "$context" -n "$namespace" run appforge-availability-probe --image=alpine:3.22 \
  --image-pull-policy=Never --restart=Never --command -- sh -c \
  'while :; do if wget -q -T 2 -O /dev/null http://appforge-api:8888/healthz; then echo ok >>/tmp/results; else echo failed >>/tmp/results; fi; sleep 0.2; done' >/dev/null
probe_started=true
"$kubectl_bin" --context "$context" -n "$namespace" wait --for=condition=Ready pod/appforge-availability-probe --timeout=60s >/dev/null
sleep 3

"$helm_bin" --kube-context "$context" upgrade appforge "$chart_dir" \
  --namespace "$namespace" -f "$temporary/values.yaml" --set "image.tag=$new_version" --atomic --timeout 10m >/dev/null
for deployment in "${deployments[@]}"; do
  "$kubectl_bin" --context "$context" -n "$namespace" rollout status "$deployment" --timeout=300s >/dev/null
done
sleep 3
"$kubectl_bin" --context "$context" -n "$namespace" exec appforge-availability-probe -- sh -c \
  'test $(grep -c ok /tmp/results) -ge 20; test ! -s /tmp/failures; ! grep -q failed /tmp/results'
upgrade_probe_count=$("$kubectl_bin" --context "$context" -n "$namespace" exec appforge-availability-probe -- sh -c 'grep -c ok /tmp/results')
if "$kubectl_bin" --context "$context" -n "$namespace" get deployment -l app.kubernetes.io/name=appforge \
  -o jsonpath='{range .items[*]}{.spec.template.spec.containers[0].image}{"\n"}{end}' | grep -Fv ":$new_version"; then
  echo "验收失败: 升级后存在未切换到 $new_version 的 AppForge Deployment" >&2
  exit 1
fi
echo "通过: Kubernetes RollingUpdate 在连续集群内健康探测下完成 ${old_version} -> ${new_version}，API 无请求失败"

"$helm_bin" --kube-context "$context" rollback appforge 1 --namespace "$namespace" --wait --timeout 10m >/dev/null
for deployment in "${deployments[@]}"; do
  "$kubectl_bin" --context "$context" -n "$namespace" rollout status "$deployment" --timeout=300s >/dev/null
done
sleep 3
"$kubectl_bin" --context "$context" -n "$namespace" exec appforge-availability-probe -- sh -c \
  'test $(grep -c ok /tmp/results) -ge 40; ! grep -q failed /tmp/results'
rollback_probe_count=$("$kubectl_bin" --context "$context" -n "$namespace" exec appforge-availability-probe -- sh -c 'grep -c ok /tmp/results')
if "$kubectl_bin" --context "$context" -n "$namespace" get deployment -l app.kubernetes.io/name=appforge \
  -o jsonpath='{range .items[*]}{.spec.template.spec.containers[0].image}{"\n"}{end}' | grep -Fv ":$old_version"; then
  echo "验收失败: 回滚后存在未恢复到 $old_version 的 AppForge Deployment" >&2
  exit 1
fi
schema_count=$("$kubectl_bin" --context "$context" -n "$namespace" exec deployment/mysql -- sh -c \
  'MYSQL_PWD=kubernetes_acceptance_mysql mysql -uappforge -Dappforge -N -e "SELECT COUNT(*) FROM sys_schema_migration WHERE version=\"20260815_113_v7_air_gapped\""')
[[ $schema_count == 1 ]] || { echo "验收失败: 应用回滚后生产 Schema 113 不存在" >&2; exit 1; }
air_gapped_table_count=$("$kubectl_bin" --context "$context" -n "$namespace" exec deployment/mysql -- sh -c \
  'MYSQL_PWD=kubernetes_acceptance_mysql mysql -uappforge -Dappforge -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=\"t_air_gapped_package\""')
[[ $air_gapped_table_count == 1 ]] || { echo "验收失败: 应用回滚后 AIR_GAPPED 表不存在" >&2; exit 1; }
"$kubectl_bin" --context "$context" -n "$namespace" get deployment appforge-api -o jsonpath='{.status.availableReplicas}' | grep -Fxq '2'
if [[ -n $report_file ]]; then
  mkdir -p "$(dirname "$report_file")"
  umask 077
  APPFORGE_K8S_OLD_VERSION=$old_version \
    APPFORGE_K8S_NEW_VERSION=$new_version \
    APPFORGE_K8S_NODE_IMAGE=$node_image \
    APPFORGE_K8S_UPGRADE_PROBES=$upgrade_probe_count \
    APPFORGE_K8S_ROLLBACK_PROBES=$rollback_probe_count \
    python3 -c '
import json, os, sys
json.dump({
  "schemaVersion": 1,
  "scenario": "local-kind-rolling-upgrade-and-application-rollback",
  "acceptanceScript": "deploy/acceptance/v7-kubernetes-upgrade.sh",
  "oldVersion": os.environ["APPFORGE_K8S_OLD_VERSION"],
  "newVersion": os.environ["APPFORGE_K8S_NEW_VERSION"],
  "kindNodeImage": os.environ["APPFORGE_K8S_NODE_IMAGE"],
  "targetDatabaseSchema": "20260815_113_v7_air_gapped",
  "upgradeSuccessfulHealthProbes": int(os.environ["APPFORGE_K8S_UPGRADE_PROBES"]),
  "rollbackSuccessfulHealthProbes": int(os.environ["APPFORGE_K8S_ROLLBACK_PROBES"]),
  "failedHealthProbes": 0,
  "verified": [
    "atomic-helm-install",
    "rolling-update-with-continuous-api-health",
    "helm-application-rollback",
    "schema-113-retained-after-rollback",
    "air-gapped-table-retained-after-rollback",
    "two-api-replicas-available-after-rollback"
  ],
  "limitations": [
    "local-kind-not-customer-target-kubernetes",
    "old-and-new-tags-use-the-same-locally-built-release-image-digest",
    "customer-csi-ingress-and-registry-not-validated"
  ]
}, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
' >"$report_file"
  chmod 0600 "$report_file"
fi
echo "通过: Helm 应用回滚恢复 ${old_version} 双副本，Schema 113 与 AIR_GAPPED 表保留且旧应用持续健康"
