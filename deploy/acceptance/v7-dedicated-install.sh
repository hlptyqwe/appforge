#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
APPFORGE_PRIVATE_ACCEPTANCE_MODE=dedicated exec "$repo_root/deploy/acceptance/v7-private-install.sh"
