#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 3 ]] || { echo "用法: $0 COMPONENT SPDX.json OUTPUT.json" >&2; exit 1; }
component=$1
sbom=$2
output=$3

[[ $component =~ ^[a-z0-9][a-z0-9-]*$ ]] || { echo "组件名不合法: $component" >&2; exit 1; }
[[ -f $sbom && ! -L $sbom ]] || { echo "SPDX 必须是普通文件: $sbom" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "缺少 jq" >&2; exit 1; }

jq -e '
  (.spdxVersion | type == "string" and startswith("SPDX-")) and
  (.packages | type == "array" and length > 0)
' "$sbom" >/dev/null || { echo "SPDX 文档缺少版本或软件包清单: $sbom" >&2; exit 1; }

jq --arg component "$component" '
  {
    schemaVersion: 1,
    component: $component,
    spdxVersion: .spdxVersion,
    documentNamespace: (.documentNamespace // ""),
    packages: [
      .packages[] |
      {
        name: .name,
        version: (.versionInfo // ""),
        supplier: (.supplier // "NOASSERTION"),
        licenseDeclared: (.licenseDeclared // "NOASSERTION"),
        licenseConcluded: (.licenseConcluded // "NOASSERTION"),
        copyrightText: (.copyrightText // "NOASSERTION")
      }
    ] | sort_by(.name, .version)
  }
' "$sbom" >"$output"
chmod 0640 "$output"
