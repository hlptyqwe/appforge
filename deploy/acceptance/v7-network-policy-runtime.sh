#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
kind_bin=${APPFORGE_KIND_BIN:-/private/tmp/appforge-v7-tools/kind}
helm_bin=${APPFORGE_HELM_BIN:-/private/tmp/appforge-v7-tools/helm}
kubectl_bin=${APPFORGE_KUBECTL_BIN:-kubectl}
calico_manifest=${APPFORGE_CALICO_MANIFEST:-/private/tmp/appforge-calico-v3.32.1.yaml}
calico_sha256=${APPFORGE_CALICO_SHA256:-a1df919d9721cf667accdc3e72848911b0cb25cfab7d2478ad0c996302c95744}
cluster=${APPFORGE_V7_NETWORK_KIND_CLUSTER:-appforge-v7-policy-${RANDOM}-$$}
context="kind-$cluster"
namespace=${APPFORGE_V7_NETWORK_NAMESPACE:-appforge-policy}
node_image=${APPFORGE_V7_KIND_NODE_IMAGE:-kindest/node:v1.32.2}
temporary=$(mktemp -d /tmp/appforge-v7-network-runtime.XXXXXX)
created_cluster=false

calico_images=(
  quay.io/calico/cni:v3.32.1
  quay.io/calico/kube-controllers:v3.32.1
  quay.io/calico/node:v3.32.1
)

cleanup() {
  set +e
  if [[ $created_cluster == true ]]; then
    "$kind_bin" delete cluster --name "$cluster" >/dev/null 2>&1 || true
  fi
  rm -rf "$temporary"
}
trap cleanup EXIT

diagnose_failure() {
  local code=$?
  trap - ERR
  set +e
  echo "--- NetworkPolicy 验收失败诊断 ---" >&2
  "$kubectl_bin" --context "$context" get nodes -o wide >&2
  "$kubectl_bin" --context "$context" get pods -A -o wide >&2
  "$kubectl_bin" --context "$context" -n "$namespace" describe pods >&2
  exit "$code"
}
trap diagnose_failure ERR

for command_path in "$kind_bin" "$helm_bin" "$kubectl_bin" docker; do
  [[ -x $command_path ]] || command -v "$command_path" >/dev/null || {
    echo "验收失败: 缺少命令 $command_path" >&2
    exit 1
  }
done
[[ -f $calico_manifest ]] || { echo "验收失败: 缺少 Calico v3.32.1 官方清单" >&2; exit 1; }
actual_calico_sha=$(shasum -a 256 "$calico_manifest" | awk '{print $1}')
[[ $actual_calico_sha == "$calico_sha256" ]] || {
  echo "验收失败: Calico 清单 SHA-256 不匹配" >&2
  exit 1
}
for image in "$node_image" alpine:3.22 "${calico_images[@]}"; do
  docker image inspect "$image" >/dev/null 2>&1 || { echo "验收失败: 缺少离线镜像 $image" >&2; exit 1; }
done

printf '%s\n' \
  'kind: Cluster' \
  'apiVersion: kind.x-k8s.io/v1alpha4' \
  'networking:' \
  '  disableDefaultCNI: true' \
  '  podSubnet: 192.168.0.0/16' \
  'nodes:' \
  '  - role: control-plane' >"$temporary/kind.yaml"

created_cluster=true
"$kind_bin" create cluster --name "$cluster" --image "$node_image" --config "$temporary/kind.yaml"
"$kind_bin" load docker-image --name "$cluster" alpine:3.22 "${calico_images[@]}" >/dev/null
"$kubectl_bin" --context "$context" apply -f "$calico_manifest" >/dev/null
"$kubectl_bin" --context "$context" -n kube-system rollout status daemonset/calico-node --timeout=300s >/dev/null
"$kubectl_bin" --context "$context" -n kube-system rollout status deployment/calico-kube-controllers --timeout=300s >/dev/null
"$kubectl_bin" --context "$context" wait --for=condition=Ready node --all --timeout=180s >/dev/null

for item in "$namespace" ingress-nginx untrusted; do
  "$kubectl_bin" --context "$context" create namespace "$item" >/dev/null
done
"$kubectl_bin" --context "$context" -n "$namespace" apply -f - <<'YAML' >/dev/null
apiVersion: v1
kind: Pod
metadata: {name: external-target, labels: {app: external-target}}
spec:
  containers:
    - name: allowed
      image: alpine:3.22
      imagePullPolicy: Never
      command: ["nc", "-lk", "-p", "8443", "-e", "/bin/cat"]
    - name: denied
      image: alpine:3.22
      imagePullPolicy: Never
      command: ["nc", "-lk", "-p", "8080", "-e", "/bin/cat"]
---
apiVersion: v1
kind: Pod
metadata:
  name: appforge-server
  labels: {app.kubernetes.io/name: appforge, app.kubernetes.io/component: api}
spec:
  containers:
    - name: server
      image: alpine:3.22
      imagePullPolicy: Never
      command: ["nc", "-lk", "-p", "8888", "-e", "/bin/cat"]
---
apiVersion: v1
kind: Pod
metadata:
  name: api-client
  labels: {app.kubernetes.io/name: appforge, app.kubernetes.io/component: api}
spec:
  containers: [{name: client, image: alpine:3.22, imagePullPolicy: Never, command: ["sleep", "3600"]}]
---
apiVersion: v1
kind: Pod
metadata:
  name: core-client
  labels: {app.kubernetes.io/name: appforge, app.kubernetes.io/component: core}
spec:
  containers: [{name: client, image: alpine:3.22, imagePullPolicy: Never, command: ["sleep", "3600"]}]
YAML
for client_namespace in ingress-nginx untrusted; do
  "$kubectl_bin" --context "$context" -n "$client_namespace" run client --image=alpine:3.22 \
    --image-pull-policy=Never --restart=Never --command -- sleep 3600 >/dev/null
done
"$kubectl_bin" --context "$context" -n "$namespace" wait --for=condition=Ready pod --all --timeout=120s >/dev/null
"$kubectl_bin" --context "$context" -n ingress-nginx wait --for=condition=Ready pod/client --timeout=60s >/dev/null
"$kubectl_bin" --context "$context" -n untrusted wait --for=condition=Ready pod/client --timeout=60s >/dev/null

external_ip=$("$kubectl_bin" --context "$context" -n "$namespace" get pod external-target -o jsonpath='{.status.podIP}')
appforge_ip=$("$kubectl_bin" --context "$context" -n "$namespace" get pod appforge-server -o jsonpath='{.status.podIP}')
printf '%s\n' \
  'networkPolicy:' \
  '  egressCIDRs: []' \
  '  externalEgressRules:' \
  '    - name: test-https' \
  '      component: api' \
  "      cidrs: [$external_ip/32]" \
  '      ports: [{protocol: TCP, port: 8443}]' >"$temporary/policy-values.yaml"

"$helm_bin" template appforge "$repo_root/deploy/helm/appforge" --show-only templates/networkpolicy.yaml \
  -f "$temporary/policy-values.yaml" \
  --set global.imageRegistry=registry.example.com/appforge \
  --set global.publicOrigin=https://appforge.example.com \
  --set image.tag=1.0.0 \
  --set ingress.host=appforge.example.com \
  --set ingress.adminHost=admin.appforge.example.com \
  --set offlineLicense.deploymentId=network-policy-runtime \
  --set offlineLicense.existingStateClaim=appforge-license-state \
  --set observability.siemWebhook=https://siem.example.com/appforge/audit >"$temporary/policies.yaml"
"$kubectl_bin" --context "$context" -n "$namespace" apply -f "$temporary/policies.yaml" >/dev/null
sleep 8

expect_allowed() {
  local source_namespace=$1 source_pod=$2 host=$3 port=$4 label=$5
  "$kubectl_bin" --context "$context" -n "$source_namespace" exec "$source_pod" -- \
    nc -z -w 3 "$host" "$port" || { echo "验收失败: 应放行但不可达: $label" >&2; exit 1; }
  echo "通过: $label"
}

expect_denied() {
  local source_namespace=$1 source_pod=$2 host=$3 port=$4 label=$5
  for _ in 1 2 3; do
    if "$kubectl_bin" --context "$context" -n "$source_namespace" exec "$source_pod" -- \
      nc -z -w 2 "$host" "$port" >/dev/null 2>&1; then
      echo "验收失败: 应拒绝但实际可达: $label" >&2
      exit 1
    fi
  done
  echo "通过: $label"
}

expect_allowed "$namespace" api-client "$external_ip" 8443 "api 组件可访问明确 CIDR 的 TCP/8443"
expect_denied "$namespace" api-client "$external_ip" 8080 "同一 CIDR 的未授权 TCP/8080 被拒绝"
expect_denied "$namespace" core-client "$external_ip" 8443 "未授权 core 组件访问 TCP/8443 被拒绝"
expect_allowed "$namespace" api-client "$appforge_ip" 8888 "AppForge 组件间通信被允许"
expect_allowed ingress-nginx client "$appforge_ip" 8888 "带授权 Namespace 标签的入口访问被允许"
expect_denied untrusted client "$appforge_ip" 8888 "未授权 Namespace 的入口访问被拒绝"

echo "通过: Calico v3.32.1/Kind 真实执行默认拒绝、组件隔离、端口白名单、内部通信和入口 Namespace 策略"
