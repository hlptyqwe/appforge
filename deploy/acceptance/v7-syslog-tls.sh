#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
report_file=${APPFORGE_SYSLOG_TLS_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-syslog-tls-20260817.json}
go_cache=${GOCACHE:-/tmp/appforge-go-build-cache}

[[ $report_file == /* ]] || { echo "验收失败: 证据路径必须是绝对路径" >&2; exit 1; }
mkdir -p "$(dirname "$report_file")" "$go_cache"
temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT
umask 077

(
  cd "$repo_root/common"
  GOCACHE="$go_cache" go test -race ./siem -count=1
)
(
  cd "$repo_root/appforge-api"
  GOCACHE="$go_cache" go test ./internal/config -count=1
)

bash -n "$repo_root/deploy/production/preflight.sh" "$repo_root/deploy/acceptance/v7-production-preflight.sh"
"$repo_root/deploy/acceptance/v7-production-preflight.sh"

docker run --rm -v "$repo_root/deploy/helm:/charts" alpine/helm:3.17.3 lint /charts/appforge --strict \
  --set global.imageRegistry=registry.example.com/appforge --set global.publicOrigin=https://appforge.example.com \
  --set image.tag=1.2.7 --set ingress.host=appforge.example.com --set ingress.adminHost=admin.appforge.example.com \
  --set offlineLicense.deploymentId=acceptance-private --set offlineLicense.existingStateClaim=appforge-license-state \
  --set observability.siemWebhook=syslog+tls://siem.customer.test:6514 >/dev/null

goos=$(go env GOOS)
goarch=$(go env GOARCH)
jq -n \
  --arg acceptedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg goos "$goos" \
  --arg goarch "$goarch" \
  '{
    schemaVersion: 1,
    evidenceType: "v7-rfc5424-syslog-tls",
    acceptedAt: $acceptedAt,
    result: "passed",
    runtime: {goos: $goos, goarch: $goarch},
    protocol: {
      messageFormat: "RFC5424",
      framing: "RFC6587-octet-counting",
      transport: "TLS",
      minimumTLSVersion: "1.2",
      priority: 134,
      facility: "local0",
      severity: "informational"
    },
    verified: [
      "real-loopback-tls-receiver",
      "custom-ca-validation",
      "two-events-two-tls-connections",
      "controlled-http-connect-egress-proxy",
      "rfc5424-header-and-structured-data",
      "rfc6587-byte-length-framing",
      "structured-data-escaping",
      "redacted-json-audit-payload",
      "bounded-message-size",
      "plaintext-syslog-rejected",
      "userinfo-path-query-and-missing-port-rejected",
      "syslog-mode-does-not-require-bearer-token",
      "compose-production-preflight",
      "helm-values-schema-and-strict-lint"
    ],
    limitations: [
      "synthetic-audit-events-only",
      "syslog-has-no-application-level-acknowledgement",
      "does-not-replace-customer-siem-parser-routing-retention-or-alert-validation"
    ]
  }' >"$temporary/report.json"
install -m 0600 "$temporary/report.json" "$report_file"

echo "通过: RFC5424 over TLS、RFC6587帧、真实TLS接收、重连、配置门禁和Helm交付"
echo "证据: $report_file"
