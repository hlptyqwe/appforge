#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

APPFORGE_DR_SCHEMA_NUMBER=113 \
  APPFORGE_DR_ACCEPTANCE_SCRIPT=deploy/acceptance/v7-appforge-schema113-dr.sh \
  APPFORGE_DR_REPORT_FILE=${APPFORGE_DR_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-appforge-schema113-dr-20260816.json} \
  "$repo_root/deploy/acceptance/v7-appforge-schema112-dr.sh"
