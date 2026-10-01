#!/bin/bash
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
exec "$here/run.sh" -demo "${1:-${DEMO_FILE:?give an absolute file path under STATS_ROOT}}"
