#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

APPFORGE_OFFLINE_INSTALL_UPGRADE_MODE=1 \
APPFORGE_OFFLINE_INSTALL_UPGRADE_VERSION=${APPFORGE_OFFLINE_UPGRADE_VERSION:-1.2.1} \
APPFORGE_OFFLINE_INSTALL_REPORT_FILE=${APPFORGE_OFFLINE_UPGRADE_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-offline-upgrade-20260815.json} \
  exec "$repo_root/deploy/acceptance/v7-offline-install.sh"
