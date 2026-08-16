#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
chart_root="$repo_root/deploy/helm"
temporary=$(mktemp -d /tmp/appforge-v7-network-policy.XXXXXX)
rendered="$temporary/rendered.yaml"
trap 'rm -rf "$temporary"' EXIT

docker image inspect alpine/helm:3.17.3 >/dev/null 2>&1 || {
  echo "验收失败: 缺少离线 Helm 验收镜像 alpine/helm:3.17.3" >&2
  exit 1
}

helm_container() {
  docker run --rm -v "$chart_root:/charts:ro" -v "$temporary:/work:ro" \
    alpine/helm:3.17.3 "$@"
}

render_args=(
  template appforge /charts/appforge
  --set global.imageRegistry=registry.example.com/appforge
  --set global.publicOrigin=https://appforge.example.com
  --set image.tag=1.0.0
  --set ingress.host=appforge.example.com
  --set ingress.adminHost=admin.appforge.example.com
  --set offlineLicense.deploymentId=network-policy-acceptance
  --set offlineLicense.existingStateClaim=appforge-license-state
  --set observability.siemWebhook=https://siem.example.com/appforge/audit
)

printf '%s\n' \
  'networkPolicy:' \
  '  egressCIDRs: []' \
  '  externalEgressRules:' \
  '    - name: siem-https' \
  '      component: api' \
  '      cidrs: [198.51.100.10/32]' \
  '      ports:' \
  '        - {protocol: TCP, port: 443}' >"$temporary/valid.yaml"

helm_container "${render_args[@]}" -f /work/valid.yaml >"$rendered"
policy=$(awk '
  /^  name: appforge-egress-siem-https$/ {capture=1}
  capture {print}
  capture && /^---$/ {exit}
' "$rendered")
[[ -n $policy ]] || { echo "验收失败: 未渲染严格 SIEM 出口策略" >&2; exit 1; }
grep -Fq 'app.kubernetes.io/component: api' <<<"$policy"
grep -Fq 'cidr: "198.51.100.10/32"' <<<"$policy"
grep -Fq 'protocol: TCP, port: 443' <<<"$policy"
grep -Fq 'ports:' <<<"$policy"
if grep -Fq '0.0.0.0/0' <<<"$policy" || grep -Fq '::/0' <<<"$policy"; then
  echo "验收失败: 严格出口策略包含全网 CIDR" >&2
  exit 1
fi

default_policy=$(awk '
  /^  name: appforge-default-deny$/ {capture=1}
  capture {print}
  capture && /^---$/ {exit}
' "$rendered")
grep -Fq 'kubernetes.io/metadata.name: monitoring' <<<"$default_policy" || {
  echo "验收失败: Prometheus 抓取来源未限定 monitoring Namespace" >&2
  exit 1
}
grep -Fq 'protocol: TCP, port: 9101' <<<"$default_policy" || {
  echo "验收失败: Prometheus 指标端口未加入入站策略" >&2
  exit 1
}
grep -Fq 'prometheus.io/port: "9101"' "$rendered"
grep -Fq 'prometheus.io/path: "/metrics"' "$rendered"
grep -Fq 'name: APPFORGE_OTLP_ENDPOINT' "$rendered"
grep -Fq 'value: ""' "$rendered"

printf '%s\n' \
  'egressProxy:' \
  '  enabled: true' \
  '  allowlist: [siem.customer.test:443]' \
  'networkPolicy:' \
  '  externalEgressRules:' \
  '    - name: proxy-https' \
  '      component: egress-proxy' \
  '      cidrs: [203.0.113.10/32]' \
  '      ports:' \
  '        - {protocol: TCP, port: 443}' >"$temporary/proxy.yaml"
helm_container "${render_args[@]}" -f /work/proxy.yaml >"$temporary/proxy-rendered.yaml"
grep -Fq 'name: appforge-egress-proxy' "$temporary/proxy-rendered.yaml"
grep -Fq 'name: HTTPS_PROXY, value: "http://appforge-egress-proxy:3128"' "$temporary/proxy-rendered.yaml"
grep -Fq 'app.kubernetes.io/component: egress-proxy' "$temporary/proxy-rendered.yaml"
grep -Fq 'cidr: "203.0.113.10/32"' "$temporary/proxy-rendered.yaml"
if helm_container template appforge /charts/appforge \
  --set global.imageRegistry=registry.example.com/appforge \
  --set global.publicOrigin=https://appforge.example.com \
  --set image.tag=1.0.0 \
  --set ingress.host=appforge.example.com \
  --set ingress.adminHost=admin.appforge.example.com \
  --set offlineLicense.deploymentId=network-policy-acceptance \
  --set offlineLicense.existingStateClaim=appforge-license-state \
  --set observability.siemWebhook=https://siem.example.com/appforge/audit \
  --set egressProxy.enabled=true \
  --set 'egressProxy.allowlist[0]=siem.customer.test:443' >/dev/null 2>&1; then
  echo "验收失败: 启用出口代理但缺少代理专属出口规则未被拒绝" >&2
  exit 1
fi

printf '%s\n' \
  'networkPolicy:' \
  '  externalEgressRules:' \
  '    - name: invalid-no-ports' \
  '      component: api' \
  '      cidrs: [198.51.100.10/32]' >"$temporary/invalid.yaml"
if helm_container lint /charts/appforge --strict -f /work/invalid.yaml \
  --set global.imageRegistry=registry.example.com/appforge \
  --set global.publicOrigin=https://appforge.example.com \
  --set image.tag=1.0.0 \
  --set ingress.host=appforge.example.com \
  --set ingress.adminHost=admin.appforge.example.com \
  --set offlineLicense.deploymentId=network-policy-acceptance \
  --set offlineLicense.existingStateClaim=appforge-license-state \
  --set observability.siemWebhook=https://siem.example.com/appforge/audit >/dev/null 2>&1; then
  echo "验收失败: 缺少端口的出口规则未被 values schema 拒绝" >&2
  exit 1
fi

helm_container "${render_args[@]}" -f /work/valid.yaml --set networkPolicy.enabled=false >"$temporary/disabled.yaml"
if grep -Fq 'kind: NetworkPolicy' "$temporary/disabled.yaml"; then
  echo "验收失败: networkPolicy.enabled=false 时仍渲染了策略"
  exit 1
fi

echo "通过: Helm 出口规则精确渲染，受限CONNECT代理仅有专属CIDR/端口出口，Prometheus与OTLP接线受控"
