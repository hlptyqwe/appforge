#!/usr/bin/env bash

set -euo pipefail

default_repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
repo_root=${APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_REPO_ROOT:-$default_repo_root}
report_file=${APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-public-repository-secret-audit-20260817.json}
self_test=${APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_SELF_TEST:-false}
scanner=${BASH_SOURCE[0]}

fail() {
  echo "公开仓库 Secret 审计失败: $*" >&2
  exit 1
}

[[ $repo_root == /* ]] || fail "仓库路径必须是绝对路径"
[[ $report_file == /* ]] || fail "证据路径必须是绝对路径"
[[ $self_test == true || $self_test == false ]] || fail "SELF_TEST 只允许 true 或 false"
for tool in git rg jq; do
  command -v "$tool" >/dev/null 2>&1 || fail "缺少工具 $tool"
done
git -C "$repo_root" rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail "目标不是 Git 工作树"

temporary=$(mktemp -d)
cleanup() { rm -rf -- "$temporary"; }
trap cleanup EXIT

# 仅使用高置信度、可公开复核的格式特征；输出永远只包含路径，不输出匹配字节。
secret_pattern='-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----|AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9_]{30,}|github_pat_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{10,}|[sr]k_live_[A-Za-z0-9]{16,}|AIza[0-9A-Za-z_-]{35}|https?://[^/@[:space:]]+:[^/@[:space:]]+@'
current_matches="$temporary/current-matches"
history_matches="$temporary/history-matches"
risky_current="$temporary/risky-current"
risky_history="$temporary/risky-history"

(
  cd "$repo_root"
  git grep -I -l -E -e "$secret_pattern" -- . 2>/dev/null || true
) | LC_ALL=C sort -u >"$current_matches"

(
  cd "$repo_root"
  history_commits=()
  while IFS= read -r commit_sha; do
    history_commits+=("$commit_sha")
  done < <(git rev-list --all)
  if ((${#history_commits[@]} > 0)); then
    git grep -I -l -E -e "$secret_pattern" "${history_commits[@]}" -- 2>/dev/null || true
  fi
) | sed 's/^[^:]*://' | LC_ALL=C sort -u >"$history_matches"

risky_name_pattern='(^|/)(\.env($|\.)|.*\.(pem|key|p12|pfx|jks|keystore)$|id_(rsa|dsa|ecdsa|ed25519)$|credentials($|\.)|secrets?\.ya?ml$)'
(
  cd "$repo_root"
  git ls-files | rg -i "$risky_name_pattern" || true
) | LC_ALL=C sort -u >"$risky_current"
(
  cd "$repo_root"
  git log --all --no-renames --name-only --pretty=format: 2>/dev/null | rg -i "$risky_name_pattern" || true
) | LC_ALL=C sort -u >"$risky_history"

is_synthetic_pattern_path() {
  case "$1" in
    appforge-ui/node_modules/* | common/observability/environment_test.go | common/siem/exporter_test.go | deploy/acceptance/v7-customer-storage-site-probe.sh | docs/enterprise/remote-apk-signing-contract.md | services/core/internal/logic/brandinghelpers_test.go)
      return 0
      ;;
    *) return 1 ;;
  esac
}

unreviewed_matches="$temporary/unreviewed-matches"
: >"$unreviewed_matches"
while IFS= read -r matching_file; do
  [[ -n $matching_file ]] || continue
  is_synthetic_pattern_path "$matching_file" || printf '%s\n' "$matching_file" >>"$unreviewed_matches"
done < <(cat "$current_matches" "$history_matches" | LC_ALL=C sort -u)

is_reviewed_path() {
  case "$1" in
    appforge-ui/.env | appforge-ui/.env.development | appforge-ui/.env.production | *.env.example | */.env.example | *.env.sample | */.env.sample | *.env.template | */.env.template | *.pub.pem | */*.pub.pem)
      return 0
      ;;
    *) return 1 ;;
  esac
}

unreviewed_risky="$temporary/unreviewed-risky"
: >"$unreviewed_risky"
while IFS= read -r risky_file; do
  [[ -n $risky_file ]] || continue
  is_reviewed_path "$risky_file" || printf '%s\n' "$risky_file" >>"$unreviewed_risky"
done < <(cat "$risky_current" "$risky_history" | LC_ALL=C sort -u)

frontend_env="$repo_root/appforge-ui/.env"
if git -C "$repo_root" ls-files --error-unmatch appforge-ui/.env >/dev/null 2>&1; then
  expected_keys=$'VITE_API_BASE_URL\nVITE_API_TIMEOUT\nVITE_APP_NAME\nVITE_ENABLE_LOG\nVITE_ENABLE_MOCK\nVITE_ROUTER_BASE'
  actual_keys=$(sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' "$frontend_env" | LC_ALL=C sort)
  [[ $actual_keys == "$expected_keys" ]] || fail "appforge-ui/.env 只能包含固定公开 VITE 配置键"
  [[ $(sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' "$frontend_env" | wc -l | tr -d ' ') == 6 ]] ||
    fail "appforge-ui/.env 存在重复或额外配置"
  rg -q '^VITE_API_BASE_URL=http://localhost:[0-9]{2,5}$' "$frontend_env" || fail "基础前端 API 地址必须是 localhost 开发地址"
  rg -q '^VITE_API_TIMEOUT=[0-9]{3,6}$' "$frontend_env" || fail "基础前端超时必须是整数"
  rg -q '^VITE_APP_NAME=[A-Za-z0-9 _.-]{1,64}$' "$frontend_env" || fail "基础前端应用名格式无效"
  rg -q '^VITE_ROUTER_BASE=/[^[:space:]]*$' "$frontend_env" || fail "基础前端路由必须是相对路径"
  rg -q '^VITE_ENABLE_MOCK=(true|false)$' "$frontend_env" || fail "基础前端 Mock 开关格式无效"
  rg -q '^VITE_ENABLE_LOG=(true|false)$' "$frontend_env" || fail "基础前端日志开关格式无效"
fi

if [[ -s $unreviewed_matches ]]; then
  LC_ALL=C sort -u "$unreviewed_matches" | sed 's/^/  - /' >&2
  if comm -12 <(LC_ALL=C sort -u "$current_matches") <(LC_ALL=C sort -u "$unreviewed_matches") | rg -q .; then
    fail "当前跟踪文件命中未登记的高置信度 Secret 模式；以上仅显示路径"
  fi
  fail "Git 历史命中未登记的高置信度 Secret 模式；以上仅显示路径"
fi
if [[ -s $unreviewed_risky ]]; then
  LC_ALL=C sort -u "$unreviewed_risky" | sed 's/^/  - /' >&2
  fail "存在未登记的高风险凭据文件名；以上仅显示路径"
fi

head_commit=$(git -C "$repo_root" rev-parse HEAD)
commit_count=$(git -C "$repo_root" rev-list --all --count)
tracked_file_count=$(git -C "$repo_root" ls-files | wc -l | tr -d ' ')
risk_named_file_count=$(cat "$risky_current" "$risky_history" | LC_ALL=C sort -u | sed '/^$/d' | wc -l | tr -d ' ')
synthetic_pattern_file_count=$(cat "$current_matches" "$history_matches" | LC_ALL=C sort -u | sed '/^$/d' | wc -l | tr -d ' ')
accepted_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
mkdir -p "$(dirname "$report_file")"
umask 077
jq -n \
  --arg acceptedAt "$accepted_at" \
  --arg gitCommit "$head_commit" \
  --argjson commitCount "$commit_count" \
  --argjson trackedFileCount "$tracked_file_count" \
  --argjson riskNamedFileCount "$risk_named_file_count" \
  --argjson syntheticPatternFileCount "$synthetic_pattern_file_count" '
  {
    schemaVersion: 1,
    evidenceType: "v7-public-repository-secret-audit",
    acceptedAt: $acceptedAt,
    result: "passed",
    source: {gitCommit: $gitCommit},
    scope: {
      currentTrackedFiles: $trackedFileCount,
      reachableGitCommits: $commitCount,
      currentAndHistoricalRiskNamedFilesReviewed: $riskNamedFileCount,
      currentAndHistoricalSyntheticPatternFilesReviewed: $syntheticPatternFileCount,
      fullReachableHistory: true
    },
    verified: [
      "current-and-full-history-high-confidence-pattern-scan",
      "only-explicit-synthetic-pattern-paths-allowed",
      "private-key-and-provider-token-pattern-policy-enforced",
      "credential-bearing-http-url-policy-enforced",
      "unreviewed-risk-named-file-absent",
      "tracked-public-vite-env-restricted"
    ],
    outputPolicy: {
      matchingBytesPrinted: false,
      findingsExposePathsOnly: true,
      credentialsIncludedInEvidence: false
    },
    dataPolicy: {
      customerDataAccessed: false,
      productionCredentialsAccessed: false
    },
    limitations: [
      "high-confidence-format-scan-not-proof-that-no-secret-of-any-kind-ever-existed",
      "does-not-replace-github-secret-scanning-or-credential-rotation",
      "development-placeholders-and-explicit-example-files-are-reviewed-by-path-policy",
      "does-not-classify-arbitrary-business-content-as-customer-data"
    ]
  }
' >"$report_file"
chmod 0600 "$report_file"

if [[ $self_test == true ]]; then
  fixture_repo="$temporary/fixture-repo"
  fixture_report="$temporary/fixture-report.json"
  mkdir -m 700 "$fixture_repo"
  git -C "$fixture_repo" init -q
  git -C "$fixture_repo" config user.name appforge-secret-audit
  git -C "$fixture_repo" config user.email appforge-secret-audit@example.invalid
  printf '%s\n' safe >"$fixture_repo/README.md"
  git -C "$fixture_repo" add README.md
  git -C "$fixture_repo" commit -qm 'safe fixture'
  APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_REPO_ROOT="$fixture_repo" \
    APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_REPORT_FILE="$fixture_report" \
    APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_SELF_TEST=false \
    "$scanner" >/dev/null

  printf 'AKIA%s\n' 'ABCDEFGHIJKLMNOP' >"$fixture_repo/current-leak.txt"
  git -C "$fixture_repo" add current-leak.txt
  if APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_REPO_ROOT="$fixture_repo" \
    APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_REPORT_FILE="$fixture_report" \
    APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_SELF_TEST=false \
    "$scanner" >"$temporary/current-negative.log" 2>&1; then
    fail "当前文件合成 Access Key 未被拒绝"
  fi
  rg -q '当前跟踪文件命中未登记的高置信度 Secret 模式' "$temporary/current-negative.log"
  git -C "$fixture_repo" commit -qm 'synthetic leak fixture'
  git -C "$fixture_repo" rm -q current-leak.txt
  git -C "$fixture_repo" commit -qm 'remove synthetic leak fixture'
  if APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_REPO_ROOT="$fixture_repo" \
    APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_REPORT_FILE="$fixture_report" \
    APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_SELF_TEST=false \
    "$scanner" >"$temporary/history-negative.log" 2>&1; then
    fail "Git 历史中的合成 Access Key 未被拒绝"
  fi
  rg -q 'Git 历史命中未登记的高置信度 Secret 模式' "$temporary/history-negative.log"

  fixture_repo="$temporary/risky-name-repo"
  mkdir -m 700 "$fixture_repo"
  git -C "$fixture_repo" init -q
  git -C "$fixture_repo" config user.name appforge-secret-audit
  git -C "$fixture_repo" config user.email appforge-secret-audit@example.invalid
  printf '%s\n' harmless >"$fixture_repo/customer-prod.key"
  git -C "$fixture_repo" add customer-prod.key
  git -C "$fixture_repo" commit -qm 'synthetic risky filename fixture'
  if APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_REPO_ROOT="$fixture_repo" \
    APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_REPORT_FILE="$fixture_report" \
    APPFORGE_PUBLIC_REPOSITORY_SECRET_AUDIT_SELF_TEST=false \
    "$scanner" >"$temporary/risky-name-negative.log" 2>&1; then
    fail "未登记高风险文件名未被拒绝"
  fi
  rg -q '存在未登记的高风险凭据文件名' "$temporary/risky-name-negative.log"
fi

report_update="$temporary/report-update.json"
jq --argjson executed "$self_test" '
  .contractNegativeTests = if $executed then {
    executed: true,
    result: "passed",
    currentTrackedSyntheticAccessKeyRejected: true,
    removedHistoricalSyntheticAccessKeyRejected: true,
    unreviewedRiskNamedFileRejected: true
  } else {
    executed: false,
    result: "not-run"
  } end
' "$report_file" >"$report_update"
install -m 0600 "$report_update" "$report_file"

echo "公开仓库 Secret 审计通过: 当前 $tracked_file_count 个跟踪文件、完整 $commit_count 个可达提交"
echo "证据: $report_file"
